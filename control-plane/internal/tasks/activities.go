package tasks

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

// AgentExecutor is the function that actually sends work to an agent.
// Set by the orchestrator when it starts up.
var AgentExecutor func(ctx context.Context, task *Task) (*Task, error)

// ExecuteAgentTaskActivity is the Temporal activity that executes an agent task.
// It delegates to whatever AgentExecutor is registered.
func ExecuteAgentTaskActivity(ctx context.Context, task *Task) (*Task, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing agent task", "taskID", task.ID, "agent", task.AssignedTo, "title", task.Title)

	if AgentExecutor == nil {
		// No executor registered -- mark as completed with a note.
		task.State = TaskCompleted
		task.Output = map[string]any{"message": "No agent executor registered. Task acknowledged."}
		return task, nil
	}

	// Report heartbeat so Temporal knows we're alive.
	activity.RecordHeartbeat(ctx, "executing")

	result, err := AgentExecutor(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	return result, nil
}
