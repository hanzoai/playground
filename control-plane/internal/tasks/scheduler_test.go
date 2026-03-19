package tasks

import (
	"testing"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/gossip"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Initialize logger for tests.
	logger.InitLogger(false)
}

func newTestScheduler() (*Scheduler, *Store, *gossip.Tracker, *events.AgentEventBus) {
	store := NewStore()
	tracker := gossip.NewTracker()
	eventBus := events.NewAgentEventBus()

	sched := NewScheduler(store, tracker, eventBus)
	sched.interval = 50 * time.Millisecond // fast ticks for testing
	return sched, store, tracker, eventBus
}

func registerAgent(t *testing.T, tracker *gossip.Tracker, agentID, spaceID, status string) {
	t.Helper()
	err := tracker.Register(gossip.AgentInfo{
		AgentID:     agentID,
		SpaceID:     spaceID,
		DID:         "did:test:" + agentID,
		DisplayName: agentID,
		Status:      status,
	})
	require.NoError(t, err)
}

func TestScheduler_AssignsTaskToIdleAgent(t *testing.T) {
	sched, store, tracker, eventBus := newTestScheduler()

	registerAgent(t, tracker, "agent-1", "s1", "online")

	task := &Task{ID: "t1", SpaceID: "s1", Title: "do work", Priority: PriorityNormal}
	require.NoError(t, store.CreateTask(task))

	// Subscribe to events to verify emission.
	ch, unsub := eventBus.Subscribe("s1")
	defer unsub()

	// Run one scheduling cycle.
	sched.schedule()

	got, err := store.GetTask("t1")
	require.NoError(t, err)
	assert.Equal(t, TaskClaimed, got.State)
	assert.Equal(t, "agent-1", got.AssignedTo)

	// Should have emitted a claimed event.
	select {
	case evt := <-ch:
		assert.Equal(t, events.AgentEventType("task.claimed"), evt.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event but got none")
	}
}

func TestScheduler_SkipsBusyAgent(t *testing.T) {
	sched, store, tracker, _ := newTestScheduler()

	registerAgent(t, tracker, "agent-busy", "s1", "busy")

	task := &Task{ID: "t1", SpaceID: "s1", Title: "work"}
	require.NoError(t, store.CreateTask(task))

	sched.schedule()

	got, _ := store.GetTask("t1")
	assert.Equal(t, TaskPending, got.State, "busy agent should not be assigned")
}

func TestScheduler_SkipsAgentWithActiveTask(t *testing.T) {
	sched, store, tracker, _ := newTestScheduler()

	registerAgent(t, tracker, "agent-1", "s1", "online")

	// Give agent-1 an active task already.
	active := &Task{ID: "active", SpaceID: "s1", Title: "already running"}
	require.NoError(t, store.CreateTask(active))
	require.NoError(t, store.ClaimTask("active", "agent-1"))
	require.NoError(t, store.StartTask("active"))

	// New pending task should not be assigned to agent-1.
	pending := &Task{ID: "pending", SpaceID: "s1", Title: "waiting"}
	require.NoError(t, store.CreateTask(pending))

	sched.schedule()

	got, _ := store.GetTask("pending")
	assert.Equal(t, TaskPending, got.State)
}

func TestScheduler_CheckTimeouts(t *testing.T) {
	sched, store, _, _ := newTestScheduler()

	task := &Task{
		ID:      "t1",
		SpaceID: "s1",
		Title:   "will timeout",
		Timeout: 10 * time.Millisecond,
	}
	require.NoError(t, store.CreateTask(task))
	require.NoError(t, store.ClaimTask("t1", "agent-1"))
	require.NoError(t, store.StartTask("t1"))

	// Wait for timeout.
	time.Sleep(20 * time.Millisecond)

	sched.checkTimeouts()

	got, _ := store.GetTask("t1")
	assert.Equal(t, TaskFailed, got.State, "should have timed out and failed")
	assert.Equal(t, "task timed out", got.Error)
}

func TestScheduler_CheckTimeouts_WithRetries(t *testing.T) {
	sched, store, _, _ := newTestScheduler()

	task := &Task{
		ID:         "t1",
		SpaceID:    "s1",
		Title:      "will timeout but retry",
		Timeout:    10 * time.Millisecond,
		MaxRetries: 1,
	}
	require.NoError(t, store.CreateTask(task))
	require.NoError(t, store.ClaimTask("t1", "agent-1"))
	require.NoError(t, store.StartTask("t1"))

	time.Sleep(20 * time.Millisecond)

	sched.checkTimeouts()

	got, _ := store.GetTask("t1")
	assert.Equal(t, TaskPending, got.State, "should be re-queued after timeout with retries")
	assert.Equal(t, 1, got.RetryCount)
}

func TestScheduler_AdvanceWorkflows_Completion(t *testing.T) {
	sched, store, _, _ := newTestScheduler()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "step 1", WorkflowID: "wf1"}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "step 2", WorkflowID: "wf1", DependsOn: []string{"t1"}}
	require.NoError(t, store.CreateTask(t1))
	require.NoError(t, store.CreateTask(t2))

	wf := &Workflow{ID: "wf1", SpaceID: "s1", Name: "pipeline", Tasks: []string{"t1", "t2"}}
	require.NoError(t, store.CreateWorkflow(wf))

	// Workflow should move to running.
	sched.advanceWorkflows()
	got, _ := store.GetWorkflow("wf1")
	assert.Equal(t, TaskRunning, got.State)

	// Complete both tasks.
	require.NoError(t, store.ClaimTask("t1", "a"))
	require.NoError(t, store.StartTask("t1"))
	require.NoError(t, store.CompleteTask("t1", nil))
	require.NoError(t, store.ClaimTask("t2", "b"))
	require.NoError(t, store.StartTask("t2"))
	require.NoError(t, store.CompleteTask("t2", nil))

	sched.advanceWorkflows()
	got, _ = store.GetWorkflow("wf1")
	assert.Equal(t, TaskCompleted, got.State)
	assert.NotNil(t, got.CompletedAt)
}

func TestScheduler_AdvanceWorkflows_Failure(t *testing.T) {
	sched, store, _, _ := newTestScheduler()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "step 1", WorkflowID: "wf1"}
	require.NoError(t, store.CreateTask(t1))

	wf := &Workflow{ID: "wf1", SpaceID: "s1", Name: "pipeline", Tasks: []string{"t1"}}
	require.NoError(t, store.CreateWorkflow(wf))

	// Fail the task.
	require.NoError(t, store.ClaimTask("t1", "a"))
	require.NoError(t, store.StartTask("t1"))
	require.NoError(t, store.FailTask("t1", "boom"))

	sched.advanceWorkflows()
	got, _ := store.GetWorkflow("wf1")
	assert.Equal(t, TaskFailed, got.State)
}

func TestScheduler_StartStop(t *testing.T) {
	sched, _, _, _ := newTestScheduler()

	sched.Start()
	// Let a few ticks run.
	time.Sleep(150 * time.Millisecond)
	sched.Stop()
	// Should not hang or panic.
}

func TestScheduler_FullLoop(t *testing.T) {
	sched, store, tracker, eventBus := newTestScheduler()

	registerAgent(t, tracker, "agent-1", "s1", "online")

	task := &Task{ID: "t1", SpaceID: "s1", Title: "work", Priority: PriorityHigh}
	require.NoError(t, store.CreateTask(task))

	ch, unsub := eventBus.Subscribe("s1")
	defer unsub()

	sched.Start()
	defer sched.Stop()

	// Wait for the scheduler to pick it up.
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatal("scheduler did not assign task within deadline")
		case <-ch:
			got, _ := store.GetTask("t1")
			if got.State == TaskClaimed {
				return // success
			}
		}
	}
}
