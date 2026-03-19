package tasks

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// AgentTaskWorkflow is the primary workflow for executing an agent task.
// It is durable -- survives server crashes and restarts.
func AgentTaskWorkflow(ctx workflow.Context, task *Task) (*Task, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: task.Timeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    int32(task.MaxRetries),
			InitialInterval:    time.Second,
			MaximumInterval:    time.Minute,
			BackoffCoefficient: 2.0,
		},
	}
	if actOpts.StartToCloseTimeout == 0 {
		actOpts.StartToCloseTimeout = time.Hour
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	var result Task
	err := workflow.ExecuteActivity(ctx, ExecuteAgentTaskActivity, task).Get(ctx, &result)
	if err != nil {
		task.State = TaskFailed
		task.Error = err.Error()
		return task, err
	}

	task.State = TaskCompleted
	task.Output = result.Output
	now := time.Now().UTC()
	task.CompletedAt = &now
	return task, nil
}

// PipelineWorkflow runs tasks sequentially -- each task starts after the previous completes.
func PipelineWorkflow(ctx workflow.Context, wf *Workflow, tasks []*Task) (*Workflow, error) {
	for i, task := range tasks {
		actOpts := workflow.ActivityOptions{
			StartToCloseTimeout: task.Timeout,
		}
		if actOpts.StartToCloseTimeout == 0 {
			actOpts.StartToCloseTimeout = time.Hour
		}
		actCtx := workflow.WithActivityOptions(ctx, actOpts)

		var result Task
		err := workflow.ExecuteActivity(actCtx, ExecuteAgentTaskActivity, task).Get(ctx, &result)
		if err != nil {
			wf.State = TaskFailed
			return wf, fmt.Errorf("step %d (%s) failed: %w", i, task.Title, err)
		}
		tasks[i] = &result
	}

	wf.State = TaskCompleted
	now := time.Now().UTC()
	wf.CompletedAt = &now
	return wf, nil
}

// FanOutWorkflow runs tasks in parallel and waits for all to complete.
func FanOutWorkflow(ctx workflow.Context, wf *Workflow, tasks []*Task) (*Workflow, error) {
	var futures []workflow.Future

	for _, task := range tasks {
		actOpts := workflow.ActivityOptions{
			StartToCloseTimeout: task.Timeout,
		}
		if actOpts.StartToCloseTimeout == 0 {
			actOpts.StartToCloseTimeout = time.Hour
		}
		actCtx := workflow.WithActivityOptions(ctx, actOpts)

		future := workflow.ExecuteActivity(actCtx, ExecuteAgentTaskActivity, task)
		futures = append(futures, future)
	}

	// Wait for all.
	var errors []string
	for i, future := range futures {
		var result Task
		if err := future.Get(ctx, &result); err != nil {
			errors = append(errors, fmt.Sprintf("task %d: %v", i, err))
		}
	}

	if len(errors) > 0 {
		wf.State = TaskFailed
		return wf, fmt.Errorf("fan-out failures: %s", strings.Join(errors, "; "))
	}

	wf.State = TaskCompleted
	now := time.Now().UTC()
	wf.CompletedAt = &now
	return wf, nil
}
