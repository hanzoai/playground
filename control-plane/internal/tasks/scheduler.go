package tasks

import (
	"time"

	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/gossip"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// Scheduler watches for ready tasks and dispatches them to available agents.
// It runs a periodic tick loop that:
//   - Assigns pending tasks to idle agents in the same space (work-stealing).
//   - Cancels tasks that exceeded their timeout.
//   - Advances workflows by unblocking tasks whose dependencies are met.
type Scheduler struct {
	store    TaskStore
	tracker  *gossip.Tracker
	eventBus *events.AgentEventBus
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewScheduler creates a scheduler that ticks every 2 seconds by default.
func NewScheduler(store TaskStore, tracker *gossip.Tracker, eventBus *events.AgentEventBus) *Scheduler {
	return &Scheduler{
		store:    store,
		tracker:  tracker,
		eventBus: eventBus,
		interval: 2 * time.Second,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the scheduling loop in a goroutine.
func (s *Scheduler) Start() {
	go s.run()
}

// Stop halts the scheduler and waits for it to finish.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *Scheduler) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.schedule()
			s.checkTimeouts()
			s.advanceWorkflows()
		}
	}
}

// schedule finds idle agents and assigns pending tasks by work-stealing.
func (s *Scheduler) schedule() {
	spaceIDs := s.store.ListSpaceIDs()

	for _, spaceID := range spaceIDs {
		agents := s.tracker.FindInSpace(spaceID)
		for _, agent := range agents {
			if agent.Status != "online" {
				continue
			}

			if s.agentHasActiveTask(agent.AgentID) {
				continue
			}

			task, err := s.store.GetNextPendingTask(spaceID, agent.AgentID)
			if err != nil {
				logger.Logger.Error().Err(err).
					Str("space_id", spaceID).
					Msg("[TaskScheduler] error getting next task")
				continue
			}
			if task == nil {
				break
			}

			logger.Logger.Info().
				Str("task_id", task.ID).
				Str("agent_id", agent.AgentID).
				Str("space_id", spaceID).
				Msg("[TaskScheduler] assigned task to agent")

			s.emitEvent("claimed", task.ID, spaceID, agent.AgentID, nil)
		}
	}
}

// agentHasActiveTask checks if the agent has a task in claimed or running state.
func (s *Scheduler) agentHasActiveTask(agentID string) bool {
	active := s.store.ListActiveTasks()
	for _, t := range active {
		if t.AssignedTo == agentID && (t.State == TaskClaimed || t.State == TaskRunning) {
			return true
		}
	}
	return false
}

// checkTimeouts cancels tasks that have exceeded their configured timeout.
func (s *Scheduler) checkTimeouts() {
	now := time.Now()
	active := s.store.ListActiveTasks()

	for _, t := range active {
		if t.State != TaskRunning || t.Timeout <= 0 || t.StartedAt == nil {
			continue
		}
		if now.Sub(*t.StartedAt) <= t.Timeout {
			continue
		}

		logger.Logger.Warn().
			Str("task_id", t.ID).
			Str("space_id", t.SpaceID).
			Dur("timeout", t.Timeout).
			Msg("[TaskScheduler] task timed out")

		if err := s.store.FailTask(t.ID, "task timed out"); err != nil {
			logger.Logger.Error().Err(err).Str("task_id", t.ID).Msg("[TaskScheduler] failed to timeout task")
		} else {
			s.emitEvent("failed", t.ID, t.SpaceID, "", map[string]any{"error": "task timed out"})
		}
	}
}

// advanceWorkflows checks workflows for completion or failure.
func (s *Scheduler) advanceWorkflows() {
	workflows := s.store.ListActiveWorkflows()

	for _, wf := range workflows {
		allCompleted := true
		anyFailed := false

		for _, taskID := range wf.Tasks {
			t, err := s.store.GetTask(taskID)
			if err != nil {
				continue
			}
			switch t.State {
			case TaskCompleted:
				// good
			case TaskFailed, TaskCancelled:
				anyFailed = true
				allCompleted = false
			default:
				allCompleted = false
			}
		}

		now := time.Now()
		if allCompleted && len(wf.Tasks) > 0 {
			wf.State = TaskCompleted
			wf.CompletedAt = &now
			if err := s.store.UpdateWorkflow(wf); err == nil {
				logger.Logger.Info().Str("workflow_id", wf.ID).Msg("[TaskScheduler] workflow completed")
			}
		} else if anyFailed {
			wf.State = TaskFailed
			wf.CompletedAt = &now
			if err := s.store.UpdateWorkflow(wf); err == nil {
				logger.Logger.Warn().Str("workflow_id", wf.ID).Msg("[TaskScheduler] workflow failed")
			}
		} else if wf.State == TaskPending {
			wf.State = TaskRunning
			_ = s.store.UpdateWorkflow(wf)
		}
	}
}

// emitEvent publishes a task event to the agent event bus.
func (s *Scheduler) emitEvent(eventType, taskID, spaceID, agentID string, data map[string]interface{}) {
	if s.eventBus == nil {
		return
	}

	eventData := map[string]interface{}{
		"task_event_type": eventType,
		"task_id":         taskID,
	}
	for k, v := range data {
		eventData[k] = v
	}

	s.eventBus.Publish(events.AgentEvent{
		Type:      events.AgentEventType("task." + eventType),
		SpaceID:   spaceID,
		AgentID:   agentID,
		Timestamp: time.Now(),
		Data:      eventData,
	})
}
