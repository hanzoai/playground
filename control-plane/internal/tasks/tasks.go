// Package tasks provides the playground task system.
//
// Types, durable execution, workflows, and activities are imported from
// github.com/hanzoai/base/plugins/tasks. This package adds the in-memory
// Store, gossip-aware Scheduler, and Gin HTTP Handlers needed by the
// playground control plane.
package tasks

import (
	basetasks "github.com/hanzoai/base/plugins/tasks"
)

// Re-export base types so the rest of the control plane imports only this package.
type Task = basetasks.Task
type TaskState = basetasks.TaskState
type TaskPriority = basetasks.TaskPriority
type TaskFilters = basetasks.TaskFilters
type Workflow = basetasks.Workflow
type DurableStore = basetasks.DurableStore
type DurableConfig = basetasks.DurableConfig
type TaskWorker = basetasks.Worker

// Re-export base constants.
const (
	TaskPending   = basetasks.TaskPending
	TaskClaimed   = basetasks.TaskClaimed
	TaskRunning   = basetasks.TaskRunning
	TaskCompleted = basetasks.TaskCompleted
	TaskFailed    = basetasks.TaskFailed
	TaskCancelled = basetasks.TaskCancelled
	TaskRetrying  = basetasks.TaskRetrying

	PriorityLow    = basetasks.PriorityLow
	PriorityNormal = basetasks.PriorityNormal
	PriorityHigh   = basetasks.PriorityHigh
	PriorityUrgent = basetasks.PriorityUrgent
)

// Re-export base sentinel errors.
var (
	ErrTaskNotFound  = basetasks.ErrTaskNotFound
	ErrAlreadyClaimed = basetasks.ErrAlreadyClaimed
	ErrInvalidTransition = basetasks.ErrInvalidTransition
)

// Re-export base constructors.
var (
	NewDurableStore   = basetasks.NewDurableStore
	DefaultDurableConfig = basetasks.DefaultDurableConfig
	NewTaskWorker     = basetasks.NewWorker
	CanTransition     = basetasks.CanTransition
)
