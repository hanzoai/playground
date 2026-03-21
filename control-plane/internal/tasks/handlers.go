package tasks

import (
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

// Handlers provides Gin HTTP handlers for the task system.
// All operations proxy to tasks.hanzo.ai via the DurableStore.
type Handlers struct {
	durable  *DurableStore
	eventBus interface{ Publish(event interface{}) } // optional SSE bus
}

// NewHandlers creates task handlers backed by the durable task service.
func NewHandlers(durable *DurableStore) *Handlers {
	return &Handlers{durable: durable}
}

// SetEventBus attaches an event bus for SSE task events.
func (h *Handlers) SetEventBus(bus interface{ Publish(event interface{}) }) {
	h.eventBus = bus
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
	Progress int `json:"progress"`
}

type nextTaskRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

// --- Task Handlers ---

// CreateTask handles POST /api/v1/spaces/:id/tasks
func (h *Handlers) CreateTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	spaceID := c.Param("id")

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orgID := orgFromContext(c)

	task := &Task{
		ID:           uuid.New().String(),
		OrgID:        orgID,
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

	if err := h.durable.SubmitTask(c.Request.Context(), task); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /api/v1/spaces/:id/tasks
func (h *Handlers) ListTasks(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	spaceID := c.Param("id")
	orgID := orgFromContext(c)
	tasks, err := h.durable.ListTasks(c.Request.Context(), spaceID, orgID)
	if err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}
	if tasks == nil {
		tasks = []*Task{}
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "count": len(tasks)})
}

// GetTask handles GET /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) GetTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	orgID := orgFromContext(c)
	state, errMsg, err := h.durable.GetTaskStatus(c.Request.Context(), taskID, orgID)
	if err != nil {
		taskError(c, http.StatusNotFound, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": taskID, "state": state, "error": errMsg})
}

// UpdateTask handles PUT /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) UpdateTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.durable.SignalTask(c.Request.Context(), taskID, "update", req, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true, "task_id": taskID})
}

// ClaimTask handles POST /api/v1/spaces/:id/tasks/:taskId/claim
func (h *Handlers) ClaimTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	var req claimTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.durable.SignalTask(c.Request.Context(), taskID, "claim", map[string]string{"agent_id": req.AgentID}, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"claimed": true, "task_id": taskID, "agent_id": req.AgentID})
}

// CompleteTask handles POST /api/v1/spaces/:id/tasks/:taskId/complete
func (h *Handlers) CompleteTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	var req completeTaskRequest
	_ = c.ShouldBindJSON(&req) // allow empty body

	if err := h.durable.SignalTask(c.Request.Context(), taskID, "complete", req.Output, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"completed": true, "task_id": taskID})
}

// FailTask handles POST /api/v1/spaces/:id/tasks/:taskId/fail
func (h *Handlers) FailTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	var req failTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.durable.SignalTask(c.Request.Context(), taskID, "fail", map[string]string{"error": req.Error}, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"failed": true, "task_id": taskID})
}

// CancelTask handles DELETE /api/v1/spaces/:id/tasks/:taskId
func (h *Handlers) CancelTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	if err := h.durable.CancelTask(c.Request.Context(), taskID, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"cancelled": true, "task_id": taskID})
}

// UpdateProgress handles POST /api/v1/spaces/:id/tasks/:taskId/progress
func (h *Handlers) UpdateProgress(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	taskID := c.Param("taskId")
	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.durable.SignalTask(c.Request.Context(), taskID, "progress", req, orgFromContext(c)); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"progress": req.Progress, "task_id": taskID})
}

// NextTask handles POST /api/v1/spaces/:id/tasks/next
func (h *Handlers) NextTask(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	spaceID := c.Param("id")
	var req nextTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.durable.GetNextTask(c.Request.Context(), spaceID, req.AgentID, orgFromContext(c))
	if err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}
	if task == nil {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	c.JSON(http.StatusOK, task)
}

// --- Workflow Handlers ---

// CreateWorkflow handles POST /api/v1/spaces/:id/workflows
func (h *Handlers) CreateWorkflow(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	spaceID := c.Param("id")

	var req struct {
		Name        string            `json:"name" binding:"required"`
		Description string            `json:"description"`
		Tasks       []createTaskRequest `json:"tasks"`
		Parallel    bool              `json:"parallel"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow must have at least one task"})
		return
	}

	wfID := uuid.New().String()
	createdBy := extractCreatedBy(c)

	orgID := orgFromContext(c)

	var taskList []*Task
	for i, td := range req.Tasks {
		task := &Task{
			ID:          uuid.New().String(),
			OrgID:       orgID,
			SpaceID:     spaceID,
			Title:       td.Title,
			Description: td.Description,
			State:       TaskPending,
			Priority:    td.Priority,
			CreatedBy:   createdBy,
			WorkflowID:  wfID,
			DependsOn:   td.DependsOn,
			Labels:      td.Labels,
			Input:       td.Input,
			MaxRetries:  td.MaxRetries,
			Metadata:    td.Metadata,
		}
		if td.TimeoutSecs > 0 {
			task.Timeout = time.Duration(td.TimeoutSecs) * time.Second
		}
		// Auto-chain sequential tasks.
		if !req.Parallel && len(task.DependsOn) == 0 && i > 0 {
			task.DependsOn = []string{taskList[i-1].ID}
		}
		taskList = append(taskList, task)
	}

	wf := &Workflow{
		ID:          wfID,
		OrgID:       orgID,
		SpaceID:     spaceID,
		Name:        req.Name,
		Description: req.Description,
		State:       TaskPending,
		CreatedBy:   createdBy,
		Metadata:    req.Metadata,
	}
	for _, t := range taskList {
		wf.Tasks = append(wf.Tasks, t.ID)
	}

	// Submit workflow to durable backend.
	if err := h.durable.SubmitWorkflow(c.Request.Context(), wf, taskList, req.Parallel); err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"workflow": wf, "tasks": taskList})
}

// ListWorkflows handles GET /api/v1/spaces/:id/workflows
func (h *Handlers) ListWorkflows(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	spaceID := c.Param("id")
	workflows, err := h.durable.ListWorkflows(c.Request.Context(), spaceID, orgFromContext(c))
	if err != nil {
		taskError(c, http.StatusInternalServerError, err)
		return
	}
	if workflows == nil {
		workflows = []*Workflow{}
	}

	c.JSON(http.StatusOK, gin.H{"workflows": workflows, "count": len(workflows)})
}

// GetWorkflow handles GET /api/v1/spaces/:id/workflows/:workflowId
func (h *Handlers) GetWorkflow(c *gin.Context) {
	if !h.requireDurable(c) {
		return
	}

	wfID := c.Param("workflowId")
	state, errMsg, err := h.durable.GetTaskStatus(c.Request.Context(), wfID, orgFromContext(c))
	if err != nil {
		taskError(c, http.StatusNotFound, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    wfID,
		"state": state,
		"error": errMsg,
	})
}

// --- Helpers ---

// requireDurableAndOrg gates all handlers — checks durable connection and org context.
func (h *Handlers) requireDurable(c *gin.Context) bool {
	if h.durable == nil || !h.durable.IsConnected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Task service not available. Ensure tasks.hanzo.ai is reachable.",
		})
		return false
	}
	if org := orgFromContext(c); org == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Organization context required. Authenticate via IAM.",
		})
		return false
	}
	return true
}

// validOrgPattern restricts org IDs to safe alphanumeric characters to prevent
// namespace injection attacks against the durable task backend.
var validOrgPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// orgFromContext extracts the IAM org from the request context.
// Returns empty string if no valid org — callers must check via requireOrg.
func orgFromContext(c *gin.Context) string {
	if org := middleware.GetOrganization(c); org != "" {
		if validOrgPattern.MatchString(org) {
			return org
		}
	}
	return ""
}

// requireOrg returns the org from context or sends 403 if missing.
func requireOrg(c *gin.Context) (string, bool) {
	org := orgFromContext(c)
	if org == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Organization context required. Authenticate via IAM."})
		return "", false
	}
	return org, true
}

func extractCreatedBy(c *gin.Context) string {
	// Prefer verified IAM identity over unverified headers.
	if user := middleware.GetIAMUser(c); user != nil {
		if user.Sub != "" {
			return user.Sub
		}
		if user.Email != "" {
			return user.Email
		}
	}
	// Fall back to agent/user headers only when IAM identity is unavailable.
	if v := c.GetHeader("X-Agent-ID"); v != "" {
		return v
	}
	return "anonymous"
}

// taskError logs the full error internally and returns a generic message to the
// client, preventing internal details (e.g., Temporal stack traces) from leaking.
func taskError(c *gin.Context, status int, err error) {
	log.Printf("task operation failed: %v", err)
	c.JSON(status, gin.H{"error": "task operation failed"})
}
