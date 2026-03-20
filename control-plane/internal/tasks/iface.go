package tasks

// TaskStore defines the interface for task persistence.
// Implemented by Store (in-memory) and PGStore (PostgreSQL).
type TaskStore interface {
	CreateTask(task *Task) error
	GetTask(id string) (*Task, error)
	UpdateTask(task *Task) error
	DeleteTask(id string) error
	ListTasks(spaceID string, filters TaskFilters) []*Task
	ClaimTask(taskID, agentID string) error
	StartTask(taskID string) error
	CompleteTask(taskID string, output map[string]any) error
	FailTask(taskID string, errMsg string) error
	CancelTask(taskID string) error
	UpdateProgress(taskID string, progress int) error
	GetNextPendingTask(spaceID string, agentID string) (*Task, error)
	CreateWorkflow(wf *Workflow) error
	GetWorkflow(id string) (*Workflow, error)
	ListWorkflows(spaceID string) []*Workflow
	UpdateWorkflow(wf *Workflow) error

	// Scheduler queries
	ListSpaceIDs() []string
	ListActiveTasks() []*Task
	ListActiveWorkflows() []*Workflow
}

// Compile-time interface checks.
var (
	_ TaskStore = (*Store)(nil)
	_ TaskStore = (*PGStore)(nil)
)
