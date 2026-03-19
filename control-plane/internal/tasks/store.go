package tasks

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrWorkflowNotFound   = errors.New("workflow not found")
	ErrTaskAlreadyExists  = errors.New("task already exists")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrTaskAlreadyClaimed = errors.New("task already claimed by another agent")
	ErrEmptyTaskID        = errors.New("task id is required")
	ErrEmptySpaceID       = errors.New("space_id is required")
	ErrEmptyTitle         = errors.New("title is required")
	ErrDependencyNotMet   = errors.New("task dependencies not met")
)

// Store is an in-memory task store with proper indexing.
// Thread-safe for concurrent access from handlers and the scheduler.
type Store struct {
	tasks     map[string]*Task     // id -> task
	workflows map[string]*Workflow // id -> workflow
	bySpace   map[string][]string  // spaceID -> task IDs
	mu        sync.RWMutex
}

// NewStore creates an empty task store.
func NewStore() *Store {
	return &Store{
		tasks:     make(map[string]*Task),
		workflows: make(map[string]*Workflow),
		bySpace:   make(map[string][]string),
	}
}

// CreateTask adds a new task to the store. The task must have ID and SpaceID set.
func (s *Store) CreateTask(task *Task) error {
	if task.ID == "" {
		return ErrEmptyTaskID
	}
	if task.SpaceID == "" {
		return ErrEmptySpaceID
	}
	if task.Title == "" {
		return ErrEmptyTitle
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return ErrTaskAlreadyExists
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.State == "" {
		task.State = TaskPending
	}

	// Copy to avoid aliasing.
	stored := *task
	s.tasks[stored.ID] = &stored
	s.bySpace[stored.SpaceID] = append(s.bySpace[stored.SpaceID], stored.ID)
	return nil
}

// GetTask returns a task by ID, or ErrTaskNotFound.
func (s *Store) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

// UpdateTask replaces a task in the store. The task must already exist.
func (s *Store) UpdateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[task.ID]; !ok {
		return ErrTaskNotFound
	}

	task.UpdatedAt = time.Now()
	stored := *task
	s.tasks[stored.ID] = &stored
	return nil
}

// DeleteTask removes a task from the store.
func (s *Store) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	// Remove from space index.
	spaceIDs := s.bySpace[t.SpaceID]
	for i, tid := range spaceIDs {
		if tid == id {
			s.bySpace[t.SpaceID] = append(spaceIDs[:i], spaceIDs[i+1:]...)
			break
		}
	}
	if len(s.bySpace[t.SpaceID]) == 0 {
		delete(s.bySpace, t.SpaceID)
	}

	delete(s.tasks, id)
	return nil
}

// ListTasks returns tasks for a space, filtered and sorted by priority descending.
func (s *Store) ListTasks(spaceID string, filters TaskFilters) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.bySpace[spaceID]
	if !ok {
		return nil
	}

	var result []*Task
	for _, id := range ids {
		t := s.tasks[id]
		if !matchesFilters(t, filters) {
			continue
		}
		cp := *t
		result = append(result, &cp)
	}

	// Sort by priority descending, then by created_at ascending.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	// Apply offset and limit.
	if filters.Offset > 0 {
		if filters.Offset >= len(result) {
			return nil
		}
		result = result[filters.Offset:]
	}
	if filters.Limit > 0 && filters.Limit < len(result) {
		result = result[:filters.Limit]
	}

	return result
}

// ClaimTask atomically transitions a task from pending to claimed for the given agent.
func (s *Store) ClaimTask(taskID, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if t.State != TaskPending {
		return ErrTaskAlreadyClaimed
	}
	if !canTransition(t.State, TaskClaimed) {
		return ErrInvalidTransition
	}

	t.State = TaskClaimed
	t.AssignedTo = agentID
	t.UpdatedAt = time.Now()
	return nil
}

// StartTask transitions a claimed task to running.
func (s *Store) StartTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if !canTransition(t.State, TaskRunning) {
		return ErrInvalidTransition
	}

	now := time.Now()
	t.State = TaskRunning
	t.StartedAt = &now
	t.UpdatedAt = now
	return nil
}

// CompleteTask transitions a running task to completed with output.
func (s *Store) CompleteTask(taskID string, output map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if !canTransition(t.State, TaskCompleted) {
		return ErrInvalidTransition
	}

	now := time.Now()
	t.State = TaskCompleted
	t.Output = output
	t.Progress = 100
	t.CompletedAt = &now
	t.UpdatedAt = now
	return nil
}

// FailTask transitions a running task to failed. If retries remain, sets state to retrying
// and re-queues as pending.
func (s *Store) FailTask(taskID string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if !canTransition(t.State, TaskFailed) {
		return ErrInvalidTransition
	}

	now := time.Now()
	t.Error = errMsg
	t.UpdatedAt = now

	if t.RetryCount < t.MaxRetries {
		t.State = TaskRetrying
		t.RetryCount++
		// Re-queue: retrying -> pending is allowed, do it inline.
		t.State = TaskPending
		t.AssignedTo = ""
		t.StartedAt = nil
		t.Progress = 0
	} else {
		t.State = TaskFailed
		t.CompletedAt = &now
	}
	return nil
}

// CancelTask transitions any non-terminal task to cancelled.
func (s *Store) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if !canTransition(t.State, TaskCancelled) {
		return ErrInvalidTransition
	}

	now := time.Now()
	t.State = TaskCancelled
	t.CompletedAt = &now
	t.UpdatedAt = now
	return nil
}

// UpdateProgress sets the progress percentage (0-100) on a running task.
func (s *Store) UpdateProgress(taskID string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if t.State != TaskRunning {
		return ErrInvalidTransition
	}

	t.Progress = progress
	t.UpdatedAt = time.Now()
	return nil
}

// GetNextPendingTask returns the highest-priority pending task in the space
// whose dependencies are all completed. This is the work-stealing primitive.
func (s *Store) GetNextPendingTask(spaceID string, agentID string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids, ok := s.bySpace[spaceID]
	if !ok {
		return nil, nil
	}

	// Collect eligible tasks.
	type candidate struct {
		task *Task
	}
	var candidates []candidate
	for _, id := range ids {
		t := s.tasks[id]
		if t.State != TaskPending {
			continue
		}
		if !s.dependenciesMet(t) {
			continue
		}
		candidates = append(candidates, candidate{task: t})
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Sort: highest priority first, oldest first within same priority.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].task.Priority != candidates[j].task.Priority {
			return candidates[i].task.Priority > candidates[j].task.Priority
		}
		return candidates[i].task.CreatedAt.Before(candidates[j].task.CreatedAt)
	})

	// Claim the best one atomically.
	best := candidates[0].task
	best.State = TaskClaimed
	best.AssignedTo = agentID
	best.UpdatedAt = time.Now()

	cp := *best
	return &cp, nil
}

// dependenciesMet returns true if all tasks in DependsOn are completed.
// Must be called with s.mu held.
func (s *Store) dependenciesMet(t *Task) bool {
	for _, depID := range t.DependsOn {
		dep, ok := s.tasks[depID]
		if !ok {
			return false
		}
		if dep.State != TaskCompleted {
			return false
		}
	}
	return true
}

// CreateWorkflow adds a new workflow to the store.
func (s *Store) CreateWorkflow(wf *Workflow) error {
	if wf.ID == "" {
		return errors.New("workflow id is required")
	}
	if wf.SpaceID == "" {
		return ErrEmptySpaceID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workflows[wf.ID]; exists {
		return errors.New("workflow already exists")
	}

	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	if wf.State == "" {
		wf.State = TaskPending
	}

	stored := *wf
	s.workflows[stored.ID] = &stored
	return nil
}

// GetWorkflow returns a workflow by ID.
func (s *Store) GetWorkflow(id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wf, ok := s.workflows[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	cp := *wf
	return &cp, nil
}

// ListWorkflows returns all workflows in a space.
func (s *Store) ListWorkflows(spaceID string) []*Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Workflow
	for _, wf := range s.workflows {
		if wf.SpaceID == spaceID {
			cp := *wf
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// UpdateWorkflow replaces a workflow in the store.
func (s *Store) UpdateWorkflow(wf *Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[wf.ID]; !ok {
		return ErrWorkflowNotFound
	}

	wf.UpdatedAt = time.Now()
	stored := *wf
	s.workflows[stored.ID] = &stored
	return nil
}

// matchesFilters checks whether a task passes all specified filters.
func matchesFilters(t *Task, f TaskFilters) bool {
	if f.State != nil && t.State != *f.State {
		return false
	}
	if f.AssignedTo != nil && t.AssignedTo != *f.AssignedTo {
		return false
	}
	if f.Priority != nil && t.Priority != *f.Priority {
		return false
	}
	if f.WorkflowID != nil && t.WorkflowID != *f.WorkflowID {
		return false
	}
	if len(f.Labels) > 0 {
		labelSet := make(map[string]struct{}, len(t.Labels))
		for _, l := range t.Labels {
			labelSet[l] = struct{}{}
		}
		for _, required := range f.Labels {
			if _, ok := labelSet[required]; !ok {
				return false
			}
		}
	}
	return true
}
