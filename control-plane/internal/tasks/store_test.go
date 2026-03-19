package tasks

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTask(spaceID string) *Task {
	return &Task{
		ID:      uuid.New().String(),
		SpaceID: spaceID,
		Title:   "test task",
		State:   TaskPending,
	}
}

func TestCreateTask(t *testing.T) {
	s := NewStore()

	task := newTestTask("space-1")
	err := s.CreateTask(task)
	require.NoError(t, err)

	got, err := s.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, TaskPending, got.State)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestCreateTask_Validation(t *testing.T) {
	s := NewStore()

	assert.Equal(t, ErrEmptyTaskID, s.CreateTask(&Task{}))
	assert.Equal(t, ErrEmptySpaceID, s.CreateTask(&Task{ID: "x"}))
	assert.Equal(t, ErrEmptyTitle, s.CreateTask(&Task{ID: "x", SpaceID: "s"}))
}

func TestCreateTask_Duplicate(t *testing.T) {
	s := NewStore()

	task := newTestTask("space-1")
	require.NoError(t, s.CreateTask(task))
	assert.Equal(t, ErrTaskAlreadyExists, s.CreateTask(task))
}

func TestGetTask_NotFound(t *testing.T) {
	s := NewStore()

	_, err := s.GetTask("nonexistent")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestUpdateTask(t *testing.T) {
	s := NewStore()

	task := newTestTask("space-1")
	require.NoError(t, s.CreateTask(task))

	task.Title = "updated title"
	require.NoError(t, s.UpdateTask(task))

	got, err := s.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated title", got.Title)
}

func TestUpdateTask_NotFound(t *testing.T) {
	s := NewStore()
	assert.Equal(t, ErrTaskNotFound, s.UpdateTask(&Task{ID: "nope"}))
}

func TestDeleteTask(t *testing.T) {
	s := NewStore()

	task := newTestTask("space-1")
	require.NoError(t, s.CreateTask(task))

	require.NoError(t, s.DeleteTask(task.ID))

	_, err := s.GetTask(task.ID)
	assert.Equal(t, ErrTaskNotFound, err)

	// Space index should be cleaned up.
	tasks := s.ListTasks("space-1", TaskFilters{})
	assert.Empty(t, tasks)
}

func TestDeleteTask_NotFound(t *testing.T) {
	s := NewStore()
	assert.Equal(t, ErrTaskNotFound, s.DeleteTask("nonexistent"))
}

func TestListTasks_PriorityOrder(t *testing.T) {
	s := NewStore()

	low := &Task{ID: "low", SpaceID: "s1", Title: "low", Priority: PriorityLow}
	high := &Task{ID: "high", SpaceID: "s1", Title: "high", Priority: PriorityHigh}
	normal := &Task{ID: "normal", SpaceID: "s1", Title: "normal", Priority: PriorityNormal}
	urgent := &Task{ID: "urgent", SpaceID: "s1", Title: "urgent", Priority: PriorityUrgent}

	require.NoError(t, s.CreateTask(low))
	require.NoError(t, s.CreateTask(high))
	require.NoError(t, s.CreateTask(normal))
	require.NoError(t, s.CreateTask(urgent))

	tasks := s.ListTasks("s1", TaskFilters{})
	require.Len(t, tasks, 4)
	assert.Equal(t, "urgent", tasks[0].ID)
	assert.Equal(t, "high", tasks[1].ID)
	assert.Equal(t, "normal", tasks[2].ID)
	assert.Equal(t, "low", tasks[3].ID)
}

func TestListTasks_FilterByState(t *testing.T) {
	s := NewStore()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "a", State: TaskPending}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "b", State: TaskPending}
	require.NoError(t, s.CreateTask(t1))
	require.NoError(t, s.CreateTask(t2))

	// Claim one.
	require.NoError(t, s.ClaimTask("t1", "agent-1"))

	state := TaskClaimed
	tasks := s.ListTasks("s1", TaskFilters{State: &state})
	require.Len(t, tasks, 1)
	assert.Equal(t, "t1", tasks[0].ID)
}

func TestListTasks_FilterByAssignedTo(t *testing.T) {
	s := NewStore()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "a"}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "b"}
	require.NoError(t, s.CreateTask(t1))
	require.NoError(t, s.CreateTask(t2))

	require.NoError(t, s.ClaimTask("t1", "agent-A"))

	agent := "agent-A"
	tasks := s.ListTasks("s1", TaskFilters{AssignedTo: &agent})
	require.Len(t, tasks, 1)
	assert.Equal(t, "t1", tasks[0].ID)
}

func TestListTasks_FilterByLabels(t *testing.T) {
	s := NewStore()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "a", Labels: []string{"bug", "urgent"}}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "b", Labels: []string{"feature"}}
	require.NoError(t, s.CreateTask(t1))
	require.NoError(t, s.CreateTask(t2))

	tasks := s.ListTasks("s1", TaskFilters{Labels: []string{"bug"}})
	require.Len(t, tasks, 1)
	assert.Equal(t, "t1", tasks[0].ID)
}

func TestListTasks_LimitOffset(t *testing.T) {
	s := NewStore()

	for i := 0; i < 10; i++ {
		require.NoError(t, s.CreateTask(&Task{
			ID:      uuid.New().String(),
			SpaceID: "s1",
			Title:   "task",
		}))
	}

	tasks := s.ListTasks("s1", TaskFilters{Limit: 3})
	assert.Len(t, tasks, 3)

	tasks = s.ListTasks("s1", TaskFilters{Offset: 8, Limit: 5})
	assert.Len(t, tasks, 2)

	tasks = s.ListTasks("s1", TaskFilters{Offset: 100})
	assert.Empty(t, tasks)
}

func TestListTasks_EmptySpace(t *testing.T) {
	s := NewStore()
	tasks := s.ListTasks("nonexistent", TaskFilters{})
	assert.Nil(t, tasks)
}

func TestClaimStartCompleteFlow(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	// Claim.
	require.NoError(t, s.ClaimTask(task.ID, "agent-1"))
	got, _ := s.GetTask(task.ID)
	assert.Equal(t, TaskClaimed, got.State)
	assert.Equal(t, "agent-1", got.AssignedTo)

	// Start.
	require.NoError(t, s.StartTask(task.ID))
	got, _ = s.GetTask(task.ID)
	assert.Equal(t, TaskRunning, got.State)
	assert.NotNil(t, got.StartedAt)

	// Progress.
	require.NoError(t, s.UpdateProgress(task.ID, 50))
	got, _ = s.GetTask(task.ID)
	assert.Equal(t, 50, got.Progress)

	// Complete.
	output := map[string]any{"result": "ok"}
	require.NoError(t, s.CompleteTask(task.ID, output))
	got, _ = s.GetTask(task.ID)
	assert.Equal(t, TaskCompleted, got.State)
	assert.Equal(t, 100, got.Progress)
	assert.NotNil(t, got.CompletedAt)
	assert.Equal(t, "ok", got.Output["result"])
}

func TestClaimStartFailRetryFlow(t *testing.T) {
	s := NewStore()

	task := &Task{
		ID:         uuid.New().String(),
		SpaceID:    "s1",
		Title:      "retriable",
		MaxRetries: 2,
	}
	require.NoError(t, s.CreateTask(task))

	// First attempt: claim, start, fail.
	require.NoError(t, s.ClaimTask(task.ID, "agent-1"))
	require.NoError(t, s.StartTask(task.ID))
	require.NoError(t, s.FailTask(task.ID, "oops"))

	got, _ := s.GetTask(task.ID)
	assert.Equal(t, TaskPending, got.State, "should be re-queued as pending")
	assert.Equal(t, 1, got.RetryCount)
	assert.Equal(t, "", got.AssignedTo, "should be unassigned after retry")

	// Second attempt: claim, start, fail.
	require.NoError(t, s.ClaimTask(task.ID, "agent-2"))
	require.NoError(t, s.StartTask(task.ID))
	require.NoError(t, s.FailTask(task.ID, "oops again"))

	got, _ = s.GetTask(task.ID)
	assert.Equal(t, TaskPending, got.State, "should be re-queued again")
	assert.Equal(t, 2, got.RetryCount)

	// Third attempt: claim, start, fail -- no more retries.
	require.NoError(t, s.ClaimTask(task.ID, "agent-3"))
	require.NoError(t, s.StartTask(task.ID))
	require.NoError(t, s.FailTask(task.ID, "final failure"))

	got, _ = s.GetTask(task.ID)
	assert.Equal(t, TaskFailed, got.State, "should be failed permanently")
	assert.Equal(t, 2, got.RetryCount, "MaxRetries=2 means 2 retries allowed, not 3")
	assert.NotNil(t, got.CompletedAt)
}

func TestCancelTask(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	require.NoError(t, s.CancelTask(task.ID))
	got, _ := s.GetTask(task.ID)
	assert.Equal(t, TaskCancelled, got.State)
	assert.NotNil(t, got.CompletedAt)
}

func TestCancelTask_CompletedIsTerminal(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))
	require.NoError(t, s.ClaimTask(task.ID, "a"))
	require.NoError(t, s.StartTask(task.ID))
	require.NoError(t, s.CompleteTask(task.ID, nil))

	err := s.CancelTask(task.ID)
	assert.Equal(t, ErrInvalidTransition, err)
}

func TestInvalidTransitions(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	// Cannot start a pending task (must claim first).
	assert.Equal(t, ErrInvalidTransition, s.StartTask(task.ID))

	// Cannot complete a pending task.
	assert.Equal(t, ErrInvalidTransition, s.CompleteTask(task.ID, nil))

	// Cannot fail a pending task.
	assert.Equal(t, ErrInvalidTransition, s.FailTask(task.ID, "x"))
}

func TestClaimTask_OnlyOnePending(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	// First claim succeeds.
	require.NoError(t, s.ClaimTask(task.ID, "agent-1"))

	// Second claim fails.
	err := s.ClaimTask(task.ID, "agent-2")
	assert.Equal(t, ErrTaskAlreadyClaimed, err)
}

func TestConcurrentClaim(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func(agentID string) {
			defer wg.Done()
			err := s.ClaimTask(task.ID, agentID)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(uuid.New().String())
	}

	wg.Wait()
	assert.Equal(t, 1, successCount, "exactly one goroutine should succeed claiming")

	got, _ := s.GetTask(task.ID)
	assert.Equal(t, TaskClaimed, got.State)
	assert.NotEmpty(t, got.AssignedTo)
}

func TestGetNextPendingTask_PriorityOrder(t *testing.T) {
	s := NewStore()

	low := &Task{ID: "low", SpaceID: "s1", Title: "low", Priority: PriorityLow}
	high := &Task{ID: "high", SpaceID: "s1", Title: "high", Priority: PriorityHigh}
	require.NoError(t, s.CreateTask(low))
	require.NoError(t, s.CreateTask(high))

	got, err := s.GetNextPendingTask("s1", "agent-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "high", got.ID)
	assert.Equal(t, TaskClaimed, got.State)

	got, err = s.GetNextPendingTask("s1", "agent-2")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "low", got.ID)
}

func TestGetNextPendingTask_RespectsDepencies(t *testing.T) {
	s := NewStore()

	parent := &Task{ID: "parent", SpaceID: "s1", Title: "parent"}
	child := &Task{ID: "child", SpaceID: "s1", Title: "child", DependsOn: []string{"parent"}, Priority: PriorityUrgent}
	independent := &Task{ID: "independent", SpaceID: "s1", Title: "independent", Priority: PriorityLow}

	require.NoError(t, s.CreateTask(parent))
	require.NoError(t, s.CreateTask(child))
	require.NoError(t, s.CreateTask(independent))

	// Child has highest priority but its dependency is not met.
	// Should get independent or parent.
	got, err := s.GetNextPendingTask("s1", "agent-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, "child", got.ID, "child should be blocked by dependency")

	// Complete parent, now child should be available.
	// Need to start parent first.
	require.NoError(t, s.StartTask("parent"))
	require.NoError(t, s.CompleteTask("parent", nil))

	got, err = s.GetNextPendingTask("s1", "agent-2")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "child", got.ID, "child should now be eligible")
}

func TestGetNextPendingTask_EmptySpace(t *testing.T) {
	s := NewStore()

	got, err := s.GetNextPendingTask("nonexistent", "agent-1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUpdateProgress_Clamping(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))
	require.NoError(t, s.ClaimTask(task.ID, "a"))
	require.NoError(t, s.StartTask(task.ID))

	require.NoError(t, s.UpdateProgress(task.ID, -10))
	got, _ := s.GetTask(task.ID)
	assert.Equal(t, 0, got.Progress)

	require.NoError(t, s.UpdateProgress(task.ID, 200))
	got, _ = s.GetTask(task.ID)
	assert.Equal(t, 100, got.Progress)
}

func TestUpdateProgress_NotRunning(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	err := s.UpdateProgress(task.ID, 50)
	assert.Equal(t, ErrInvalidTransition, err)
}

// --- Workflow Tests ---

func TestCreateWorkflow(t *testing.T) {
	s := NewStore()

	wf := &Workflow{
		ID:      uuid.New().String(),
		SpaceID: "s1",
		Name:    "deploy pipeline",
	}
	require.NoError(t, s.CreateWorkflow(wf))

	got, err := s.GetWorkflow(wf.ID)
	require.NoError(t, err)
	assert.Equal(t, wf.ID, got.ID)
	assert.Equal(t, TaskPending, got.State)
}

func TestCreateWorkflow_Validation(t *testing.T) {
	s := NewStore()

	err := s.CreateWorkflow(&Workflow{})
	assert.Error(t, err)

	err = s.CreateWorkflow(&Workflow{ID: "x"})
	assert.Equal(t, ErrEmptySpaceID, err)
}

func TestGetWorkflow_NotFound(t *testing.T) {
	s := NewStore()

	_, err := s.GetWorkflow("nonexistent")
	assert.Equal(t, ErrWorkflowNotFound, err)
}

func TestListWorkflows(t *testing.T) {
	s := NewStore()

	wf1 := &Workflow{ID: "wf1", SpaceID: "s1", Name: "first"}
	wf2 := &Workflow{ID: "wf2", SpaceID: "s1", Name: "second"}
	wf3 := &Workflow{ID: "wf3", SpaceID: "s2", Name: "other space"}

	require.NoError(t, s.CreateWorkflow(wf1))
	require.NoError(t, s.CreateWorkflow(wf2))
	require.NoError(t, s.CreateWorkflow(wf3))

	result := s.ListWorkflows("s1")
	assert.Len(t, result, 2)

	result = s.ListWorkflows("s2")
	assert.Len(t, result, 1)

	result = s.ListWorkflows("nonexistent")
	assert.Empty(t, result)
}

func TestUpdateWorkflow(t *testing.T) {
	s := NewStore()

	wf := &Workflow{ID: "wf1", SpaceID: "s1", Name: "original"}
	require.NoError(t, s.CreateWorkflow(wf))

	wf.Name = "updated"
	require.NoError(t, s.UpdateWorkflow(wf))

	got, err := s.GetWorkflow("wf1")
	require.NoError(t, err)
	assert.Equal(t, "updated", got.Name)
}

func TestUpdateWorkflow_NotFound(t *testing.T) {
	s := NewStore()
	err := s.UpdateWorkflow(&Workflow{ID: "nonexistent"})
	assert.Equal(t, ErrWorkflowNotFound, err)
}

// --- Copy isolation tests ---

func TestGetTask_ReturnsCopy(t *testing.T) {
	s := NewStore()

	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))

	got1, _ := s.GetTask(task.ID)
	got1.Title = "mutated"

	got2, _ := s.GetTask(task.ID)
	assert.Equal(t, "test task", got2.Title, "store should not be affected by caller mutation")
}

func TestListTasks_FilterByWorkflowID(t *testing.T) {
	s := NewStore()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "a", WorkflowID: "wf-1"}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "b", WorkflowID: "wf-2"}
	t3 := &Task{ID: "t3", SpaceID: "s1", Title: "c", WorkflowID: "wf-1"}
	require.NoError(t, s.CreateTask(t1))
	require.NoError(t, s.CreateTask(t2))
	require.NoError(t, s.CreateTask(t3))

	wfID := "wf-1"
	tasks := s.ListTasks("s1", TaskFilters{WorkflowID: &wfID})
	assert.Len(t, tasks, 2)

	// Time ordering: oldest first within same priority.
	for _, tt := range tasks {
		assert.Equal(t, "wf-1", tt.WorkflowID)
	}
}

func TestDeleteTask_MultipleInSpace(t *testing.T) {
	s := NewStore()

	t1 := &Task{ID: "t1", SpaceID: "s1", Title: "first"}
	t2 := &Task{ID: "t2", SpaceID: "s1", Title: "second"}
	require.NoError(t, s.CreateTask(t1))
	require.NoError(t, s.CreateTask(t2))

	require.NoError(t, s.DeleteTask("t1"))

	tasks := s.ListTasks("s1", TaskFilters{})
	require.Len(t, tasks, 1)
	assert.Equal(t, "t2", tasks[0].ID)
}

// TestTimestamps verifies that CreatedAt and UpdatedAt are set and updated correctly.
func TestTimestamps(t *testing.T) {
	s := NewStore()

	before := time.Now()
	task := newTestTask("s1")
	require.NoError(t, s.CreateTask(task))
	after := time.Now()

	got, _ := s.GetTask(task.ID)
	assert.True(t, got.CreatedAt.After(before) || got.CreatedAt.Equal(before))
	assert.True(t, got.CreatedAt.Before(after) || got.CreatedAt.Equal(after))
	assert.Equal(t, got.CreatedAt, got.UpdatedAt)

	// Update should change UpdatedAt.
	time.Sleep(time.Millisecond) // ensure clock tick
	got.Title = "changed"
	require.NoError(t, s.UpdateTask(got))

	got2, _ := s.GetTask(task.ID)
	assert.True(t, got2.UpdatedAt.After(got2.CreatedAt))
}
