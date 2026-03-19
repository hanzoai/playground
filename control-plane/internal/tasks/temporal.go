package tasks

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// TemporalStore implements durable task execution via the Temporal SDK.
type TemporalStore struct {
	Client    client.Client
	namespace string
	connected bool
}

// NewTemporalStore connects to a Temporal server.
func NewTemporalStore(addr, namespace string) (*TemporalStore, error) {
	c, err := client.Dial(client.Options{
		HostPort:  addr,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Temporal at %s: %w", addr, err)
	}
	return &TemporalStore{Client: c, namespace: namespace, connected: true}, nil
}

// Close shuts down the Temporal client.
func (ts *TemporalStore) Close() {
	if ts.Client != nil {
		ts.Client.Close()
	}
}

// SubmitTask starts a workflow execution for a task.
func (ts *TemporalStore) SubmitTask(ctx context.Context, task *Task) error {
	opts := client.StartWorkflowOptions{
		ID:        task.ID,
		TaskQueue: task.SpaceID, // each space = a task queue
	}
	if task.Timeout > 0 {
		opts.WorkflowExecutionTimeout = task.Timeout
	}
	if task.MaxRetries > 0 {
		opts.RetryPolicy = &temporal.RetryPolicy{
			MaximumAttempts:    int32(task.MaxRetries),
			InitialInterval:    time.Second,
			MaximumInterval:    time.Minute,
			BackoffCoefficient: 2.0,
		}
	}

	we, err := ts.Client.ExecuteWorkflow(ctx, opts, AgentTaskWorkflow, task)
	if err != nil {
		return fmt.Errorf("failed to submit task workflow: %w", err)
	}
	task.State = TaskRunning
	_ = we // workflow execution handle
	return nil
}

// GetTaskStatus queries a running workflow for its current state.
func (ts *TemporalStore) GetTaskStatus(ctx context.Context, taskID string) (*Task, error) {
	desc, err := ts.Client.DescribeWorkflowExecution(ctx, taskID, "")
	if err != nil {
		return nil, err
	}

	task := &Task{
		ID: taskID,
	}

	// Map Temporal workflow status to our TaskState.
	info := desc.WorkflowExecutionInfo
	switch info.GetStatus().String() {
	case "Running":
		task.State = TaskRunning
	case "Completed":
		task.State = TaskCompleted
	case "Failed":
		task.State = TaskFailed
	case "Canceled", "Cancelled":
		task.State = TaskCancelled
	case "TimedOut":
		task.State = TaskFailed
		task.Error = "timed out"
	default:
		task.State = TaskPending
	}

	return task, nil
}

// CancelTask cancels a running workflow.
func (ts *TemporalStore) CancelTask(ctx context.Context, taskID string) error {
	return ts.Client.CancelWorkflow(ctx, taskID, "")
}

// SignalTask sends a signal to a running workflow.
func (ts *TemporalStore) SignalTask(ctx context.Context, taskID, signalName string, data interface{}) error {
	return ts.Client.SignalWorkflow(ctx, taskID, "", signalName, data)
}

// IsConnected reports whether the Temporal store has an active connection.
func (ts *TemporalStore) IsConnected() bool {
	return ts.connected
}
