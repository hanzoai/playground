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
	store    *Store
	tracker  *gossip.Tracker
	eventBus *events.AgentEventBus
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewScheduler creates a scheduler that ticks every 2 seconds by default.
func NewScheduler(store *Store, tracker *gossip.Tracker, eventBus *events.AgentEventBus) *Scheduler {
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

// Stop gracefully stops the scheduler and waits for the loop to exit.
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
	// Get all spaces that have pending tasks.
	s.store.mu.RLock()
	spaceIDs := make([]string, 0, len(s.store.bySpace))
	for sid := range s.store.bySpace {
		spaceIDs = append(spaceIDs, sid)
	}
	s.store.mu.RUnlock()

	for _, spaceID := range spaceIDs {
		agents := s.tracker.FindInSpace(spaceID)
		for _, agent := range agents {
			if agent.Status != "online" {
				continue
			}

			// Check if this agent already has a claimed or running task.
			if s.agentHasActiveTask(agent.AgentID) {
				continue
			}

			// Work-steal: get highest priority pending task with met dependencies.
			task, err := s.store.GetNextPendingTask(spaceID, agent.AgentID)
			if err != nil {
				logger.Logger.Error().Err(err).
					Str("space_id", spaceID).
					Msg("[TaskScheduler] error getting next task")
				continue
			}
			if task == nil {
				break // no more pending tasks in this space
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
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	for _, t := range s.store.tasks {
		if t.AssignedTo == agentID && (t.State == TaskClaimed || t.State == TaskRunning) {
			return true
		}
	}
	return false
}

// checkTimeouts cancels tasks that have exceeded their configured timeout.
func (s *Scheduler) checkTimeouts() {
	now := time.Now()

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	for _, t := range s.store.tasks {
		if t.State != TaskRunning {
			continue
		}
		if t.Timeout <= 0 {
			continue
		}
		if t.StartedAt == nil {
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

		t.Error = "task timed out"
		t.UpdatedAt = now

		if t.RetryCount < t.MaxRetries {
			t.State = TaskRetrying
			t.RetryCount++
			t.State = TaskPending
			t.AssignedTo = ""
			t.StartedAt = nil
			t.Progress = 0
			s.emitEvent("retrying", t.ID, t.SpaceID, "", nil)
		} else {
			t.State = TaskFailed
			t.CompletedAt = &now
			s.emitEvent("failed", t.ID, t.SpaceID, "", map[string]any{"error": "task timed out"})
		}
	}
}

// advanceWorkflows checks workflows for tasks that are now unblocked.
func (s *Scheduler) advanceWorkflows() {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	for _, wf := range s.store.workflows {
		if wf.State == TaskCompleted || wf.State == TaskFailed || wf.State == TaskCancelled {
			continue
		}

		allCompleted := true
		anyFailed := false

		for _, taskID := range wf.Tasks {
			t, ok := s.store.tasks[taskID]
			if !ok {
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
			wf.UpdatedAt = now
			logger.Logger.Info().
				Str("workflow_id", wf.ID).
				Msg("[TaskScheduler] workflow completed")
		} else if anyFailed {
			wf.State = TaskFailed
			wf.CompletedAt = &now
			wf.UpdatedAt = now
			logger.Logger.Warn().
				Str("workflow_id", wf.ID).
				Msg("[TaskScheduler] workflow failed")
		} else if wf.State == TaskPending {
			// Start the workflow if it has tasks.
			wf.State = TaskRunning
			wf.UpdatedAt = now
		}
	}
}

// emitEvent publishes a task event to the agent event bus.
func (s *Scheduler) emitEvent(eventType, taskID, spaceID, agentID string, data map[string]any) {
	if s.eventBus == nil {
		return
	}

	eventData := map[string]interface{}{
		"task_event_type": eventType,
		"task_id":         taskID,
	}
	if data != nil {
		for k, v := range data {
			eventData[k] = v
		}
	}

	s.eventBus.Publish(events.AgentEvent{
		Type:      events.AgentEventType("task." + eventType),
		SpaceID:   spaceID,
		AgentID:   agentID,
		Timestamp: time.Now(),
		Data:      eventData,
	})
}
