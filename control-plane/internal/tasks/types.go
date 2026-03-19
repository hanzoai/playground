package tasks

import "time"

// TaskState represents the lifecycle state of a task.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskClaimed   TaskState = "claimed"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
	TaskRetrying  TaskState = "retrying"
)

// validTransitions defines which state transitions are allowed.
var validTransitions = map[TaskState]map[TaskState]bool{
	TaskPending:   {TaskClaimed: true, TaskCancelled: true},
	TaskClaimed:   {TaskRunning: true, TaskCancelled: true},
	TaskRunning:   {TaskCompleted: true, TaskFailed: true, TaskCancelled: true},
	TaskFailed:    {TaskRetrying: true, TaskCancelled: true},
	TaskRetrying:  {TaskPending: true, TaskCancelled: true},
	TaskCompleted: {},
	TaskCancelled: {},
}

// canTransition reports whether moving from one state to another is allowed.
func canTransition(from, to TaskState) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// TaskPriority controls scheduling order. Higher values are scheduled first.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityUrgent TaskPriority = 3
)

// Task is a durable work item assigned to an agent.
type Task struct {
	ID           string            `json:"id"`
	SpaceID      string            `json:"space_id"`
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	State        TaskState         `json:"state"`
	Priority     TaskPriority      `json:"priority"`
	AssignedTo   string            `json:"assigned_to,omitempty"`    // agent ID
	CreatedBy    string            `json:"created_by"`               // agent or human ID
	WorkflowID   string            `json:"workflow_id,omitempty"`    // parent workflow
	ParentTaskID string            `json:"parent_task_id,omitempty"` // for subtasks
	DependsOn    []string          `json:"depends_on,omitempty"`     // task IDs that must complete first
	Labels       []string          `json:"labels,omitempty"`
	Input        map[string]any    `json:"input,omitempty"`
	Output       map[string]any    `json:"output,omitempty"`
	Error        string            `json:"error,omitempty"`
	Progress     int               `json:"progress"`     // 0-100
	MaxRetries   int               `json:"max_retries"`
	RetryCount   int               `json:"retry_count"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Workflow chains tasks into a DAG.
type Workflow struct {
	ID          string            `json:"id"`
	SpaceID     string            `json:"space_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	State       TaskState         `json:"state"`
	Tasks       []string          `json:"tasks"`      // ordered task IDs
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskEvent is emitted when task state changes.
type TaskEvent struct {
	Type      string    `json:"type"` // "created", "claimed", "progress", "completed", "failed"
	TaskID    string    `json:"task_id"`
	SpaceID   string    `json:"space_id"`
	AgentID   string    `json:"agent_id,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// TaskFilters controls listing and searching.
type TaskFilters struct {
	State      *TaskState    `json:"state,omitempty"`
	AssignedTo *string       `json:"assigned_to,omitempty"`
	Priority   *TaskPriority `json:"priority,omitempty"`
	Labels     []string      `json:"labels,omitempty"`
	WorkflowID *string       `json:"workflow_id,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	Offset     int           `json:"offset,omitempty"`
}
