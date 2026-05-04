// Package tasks provides the playground task system.
//
// All task state is managed by tasks.hanzo.ai (github.com/hanzoai/tasks).
// Types, durable execution, workflows, and activities are imported from
// github.com/hanzoai/base/plugins/tasks. This package adds Gin HTTP
// handlers that proxy to the durable task service.
package tasks

import (
	basetasks "github.com/hanzoai/base/plugins/tasks"
)

// Re-export base types.
type Task = basetasks.Task
type TaskState = basetasks.TaskState
type TaskPriority = basetasks.TaskPriority
type Workflow = basetasks.Workflow
type DurableStore = basetasks.DurableStore
type DurableConfig = basetasks.DurableConfig
type TaskWorker = basetasks.Worker

// Re-export constants.
const (
	TaskPending   = basetasks.TaskPending
	TaskClaimed   = basetasks.TaskClaimed
	TaskRunning   = basetasks.TaskRunning
	TaskCompleted = basetasks.TaskCompleted
	TaskFailed    = basetasks.TaskFailed
	TaskCancelled = basetasks.TaskCancelled
)

// Re-export constructors.
var (
	NewDurableStore      = basetasks.NewDurableStore
	DefaultDurableConfig = basetasks.DefaultDurableConfig
	NewTaskWorker        = basetasks.NewWorker
)
