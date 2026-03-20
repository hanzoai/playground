package tasks

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers provides Gin HTTP handlers for the task system.
type Handlers struct {
	store     TaskStore
	scheduler *Scheduler
	durable   *DurableStore // nil if durable tasks not configured
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(store TaskStore, scheduler *Scheduler) *Handlers {
	return &Handlers{store: store, scheduler: scheduler}
}

// SetDurable attaches a durable store (tasks.hanzo.ai) for cloud task execution.
// When set, CreateTask and CancelTask will attempt durable execution first,
// falling back to the in-memory store on failure.
func (h *Handlers) SetDurable(ds *DurableStore) {
	h.durable = ds
}

// --- Request / Response types ---

type createTaskRequest struct {
	Title        string            `json:"title" binding:"required"`
	Description  string            `json:"description"`
	Priority     TaskPriority      `json:"priority"`
	AssignedTo   string            `json:"assigned_to"`
	WorkflowID   string            `json:"workflow_id"`
	ParentTaskID string            `json:"parent_task_id"`
	DependsOn    []string          `json:"depends_on"`
	Labels       []string          `json:"labels"`
	Input        map[string]any    `json:"input"`
	MaxRetries   int               `json:"max_retries"`
	TimeoutSecs  int               `json:"timeout_secs"`
	Metadata     map[string]string `json:"metadata"`
}

type updateTaskRequest struct {
	Title       *string           `json:"title"`
	Description *string           `json:"description"`
	Priority    *TaskPriority     `json:"priority"`
	Labels      []string          `json:"labels"`
	Metadata    map[string]string `json:"metadata"`
}

type claimTaskRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type completeTaskRequest struct {
	Output map[string]any `json:"output"`
}

type failTaskRequest struct {
	Error string `json:"error" binding:"required"`
}

type progressRequest struct {
	Progress int `json:"progress" binding:"required"`
}

type nextTaskRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type createWorkflowRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Tasks       []createTaskRequest `json:"tasks" binding:"required"`
	Metadata    map[string]string `json:"metadata"`
}

// --- Task Handlers ---

// CreateTask handles POST /api/v1/spaces/:id/tasks
func (h *Handlers) CreateTask(c *gin.Context) {
	spaceID := c.Param("id")

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task := &Task{
		ID:           uuid.New().String(),
		SpaceID:      spaceID,
		Title:        req.Title,
		Description:  req.Description,
		State:        TaskPending,
		Priority:     req.Priority,
		AssignedTo:   req.AssignedTo,
		CreatedBy:    extractCreatedBy(c),
		WorkflowID:   req.WorkflowID,
		ParentTaskID: req.ParentTaskID,
		DependsOn:    req.DependsOn,
		Labels:       req.Labels,
		Input:        req.Input,
		MaxRetries:   req.MaxRetries,
		Metadata:     req.Metadata,
	}

	if req.TimeoutSecs > 0 {
		task.Timeout = time.Duration(req.TimeoutSecs) * time.Second
	}

	if err := h.store.CreateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// If durable task service is connected, submit as a durable workflow.
	if h.durable != nil && h.durable.IsConnected() {
		if err := h.durable.SubmitTask(c.Request.Context(), task); err != nil {
			log.Printf("Durable task submit failed, using in-memory: %v", err)
		}
	}

	h.emitEvent("created", task)
	c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /api/v1/spaces/:id/tasks
func (h *Handlers) ListTasks(c *gin.Context) {
	spaceID := c.Param("id")
	filters := parseTaskFilters(c)

	tasks := h.store.ListTasks(spaceID, filters)
	if tasks == nil {
		tasks = make([]*Task, 0)
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "count": len(tasks)})
}

// GetTask handles GET /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) GetTask(c *gin.Context) {
	taskID := c.Param("taskId")

	task, err := h.store.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// UpdateTask handles PUT /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) UpdateTask(c *gin.Context) {
	taskID := c.Param("taskId")

	task, err := h.store.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.Labels != nil {
		task.Labels = req.Labels
	}
	if req.Metadata != nil {
		task.Metadata = req.Metadata
	}

	if err := h.store.UpdateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// ClaimTask handles POST /api/v1/spaces/:id/tasks/:taskId/claim
func (h *Handlers) ClaimTask(c *gin.Context) {
	taskID := c.Param("taskId")

	var req claimTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.ClaimTask(taskID, req.AgentID); err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrAlreadyClaimed || err == ErrInvalidTransition {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.store.GetTask(taskID)
	h.emitEvent("claimed", task)
	c.JSON(http.StatusOK, task)
}

// CompleteTask handles POST /api/v1/spaces/:id/tasks/:taskId/complete
func (h *Handlers) CompleteTask(c *gin.Context) {
	taskID := c.Param("taskId")

	var req completeTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for tasks with no output.
		req = completeTaskRequest{}
	}

	// Auto-promote claimed → running so agents can claim+complete in one step.
	if t, err := h.store.GetTask(taskID); err == nil && t.State == TaskClaimed {
		_ = h.store.StartTask(taskID)
	}

	if err := h.store.CompleteTask(taskID, req.Output); err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidTransition {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.store.GetTask(taskID)
	h.emitEvent("completed", task)
	c.JSON(http.StatusOK, task)
}

// FailTask handles POST /api/v1/spaces/:id/tasks/:taskId/fail
func (h *Handlers) FailTask(c *gin.Context) {
	taskID := c.Param("taskId")

	var req failTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-promote claimed → running so fail works after claim.
	if t, err := h.store.GetTask(taskID); err == nil && t.State == TaskClaimed {
		_ = h.store.StartTask(taskID)
	}

	if err := h.store.FailTask(taskID, req.Error); err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidTransition {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.store.GetTask(taskID)
	h.emitEvent("failed", task)
	c.JSON(http.StatusOK, task)
}

// UpdateProgress handles POST /api/v1/spaces/:id/tasks/:taskId/progress
func (h *Handlers) UpdateProgress(c *gin.Context) {
	taskID := c.Param("taskId")

	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.UpdateProgress(taskID, req.Progress); err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidTransition {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.store.GetTask(taskID)
	h.emitEvent("progress", task)
	c.JSON(http.StatusOK, task)
}

// CancelTask handles DELETE /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) CancelTask(c *gin.Context) {
	taskID := c.Param("taskId")

	if err := h.store.CancelTask(taskID); err != nil {
		status := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidTransition {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// Also cancel the durable workflow if connected.
	if h.durable != nil && h.durable.IsConnected() {
		if err := h.durable.CancelTask(c.Request.Context(), taskID); err != nil {
			log.Printf("Durable task cancel failed for task %s: %v", taskID, err)
		}
	}

	task, _ := h.store.GetTask(taskID)
	h.emitEvent("cancelled", task)
	c.JSON(http.StatusOK, task)
}

// NextTask handles POST /api/v1/spaces/:id/tasks/next
func (h *Handlers) NextTask(c *gin.Context) {
	spaceID := c.Param("id")

	var req nextTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.store.GetNextPendingTask(spaceID, req.AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if task == nil {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	h.emitEvent("claimed", task)
	c.JSON(http.StatusOK, task)
}

// --- Workflow Handlers ---

// CreateWorkflow handles POST /api/v1/spaces/:id/workflows
func (h *Handlers) CreateWorkflow(c *gin.Context) {
	spaceID := c.Param("id")

	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wfID := uuid.New().String()
	createdBy := extractCreatedBy(c)

	// Create all tasks first, linking them to the workflow.
	taskIDs := make([]string, 0, len(req.Tasks))
	for i, tr := range req.Tasks {
		task := &Task{
			ID:          uuid.New().String(),
			SpaceID:     spaceID,
			Title:       tr.Title,
			Description: tr.Description,
			State:       TaskPending,
			Priority:    tr.Priority,
			CreatedBy:   createdBy,
			WorkflowID:  wfID,
			DependsOn:   tr.DependsOn,
			Labels:      tr.Labels,
			Input:       tr.Input,
			MaxRetries:  tr.MaxRetries,
			Metadata:    tr.Metadata,
		}
		if tr.TimeoutSecs > 0 {
			task.Timeout = time.Duration(tr.TimeoutSecs) * time.Second
		}

		// Chain dependency: each task depends on the previous unless explicit deps are set.
		if len(task.DependsOn) == 0 && i > 0 {
			task.DependsOn = []string{taskIDs[i-1]}
		}

		if err := h.store.CreateTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		taskIDs = append(taskIDs, task.ID)
	}

	wf := &Workflow{
		ID:          wfID,
		SpaceID:     spaceID,
		Name:        req.Name,
		Description: req.Description,
		State:       TaskPending,
		Tasks:       taskIDs,
		CreatedBy:   createdBy,
		Metadata:    req.Metadata,
	}

	if err := h.store.CreateWorkflow(wf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, wf)
}

// ListWorkflows handles GET /api/v1/spaces/:id/workflows
func (h *Handlers) ListWorkflows(c *gin.Context) {
	spaceID := c.Param("id")

	workflows := h.store.ListWorkflows(spaceID)
	if workflows == nil {
		workflows = make([]*Workflow, 0)
	}

	c.JSON(http.StatusOK, gin.H{"workflows": workflows, "count": len(workflows)})
}

// GetWorkflow handles GET /api/v1/spaces/:id/workflows/:workflowId
func (h *Handlers) GetWorkflow(c *gin.Context) {
	wfID := c.Param("workflowId")

	wf, err := h.store.GetWorkflow(wfID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Attach task details.
	var tasks []*Task
	for _, tid := range wf.Tasks {
		t, terr := h.store.GetTask(tid)
		if terr == nil {
			tasks = append(tasks, t)
		}
	}
	if tasks == nil {
		tasks = make([]*Task, 0)
	}

	c.JSON(http.StatusOK, gin.H{"workflow": wf, "tasks": tasks})
}

// --- Helpers ---

func extractCreatedBy(c *gin.Context) string {
	// Try agent_id header, then fallback.
	if agentID := c.GetHeader("X-Agent-ID"); agentID != "" {
		return agentID
	}
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		return userID
	}
	return "anonymous"
}

func parseTaskFilters(c *gin.Context) TaskFilters {
	var f TaskFilters

	if s := c.Query("state"); s != "" {
		state := TaskState(s)
		f.State = &state
	}
	if a := c.Query("assigned_to"); a != "" {
		f.AssignedTo = &a
	}
	if p := c.Query("priority"); p != "" {
		if pv, err := strconv.Atoi(p); err == nil {
			priority := TaskPriority(pv)
			f.Priority = &priority
		}
	}
	if w := c.Query("workflow_id"); w != "" {
		f.WorkflowID = &w
	}
	if l := c.Query("limit"); l != "" {
		if lv, err := strconv.Atoi(l); err == nil {
			f.Limit = lv
		}
	}
	if o := c.Query("offset"); o != "" {
		if ov, err := strconv.Atoi(o); err == nil {
			f.Offset = ov
		}
	}

	return f
}

// emitEvent emits a task event via the scheduler's event bus.
func (h *Handlers) emitEvent(eventType string, task *Task) {
	if h.scheduler == nil {
		return
	}
	h.scheduler.emitEvent(eventType, task.ID, task.SpaceID, task.AssignedTo, nil)
}
