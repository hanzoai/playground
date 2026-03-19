package tasks

import (
	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

// TaskWorker runs a Temporal worker inside the playground process.
// It registers our workflows and activities and polls the task queue.
type TaskWorker struct {
	worker sdkworker.Worker
	queue  string
}

// NewTaskWorker creates a worker for a specific task queue (typically a space ID).
func NewTaskWorker(c client.Client, queue string) *TaskWorker {
	w := sdkworker.New(c, queue, sdkworker.Options{})

	// Register workflows.
	w.RegisterWorkflow(AgentTaskWorkflow)
	w.RegisterWorkflow(PipelineWorkflow)
	w.RegisterWorkflow(FanOutWorkflow)

	// Register activities.
	w.RegisterActivity(ExecuteAgentTaskActivity)

	return &TaskWorker{worker: w, queue: queue}
}

// Start begins polling Temporal for tasks.
func (tw *TaskWorker) Start() error {
	return tw.worker.Start()
}

// Stop gracefully stops the worker.
func (tw *TaskWorker) Stop() {
	tw.worker.Stop()
}
