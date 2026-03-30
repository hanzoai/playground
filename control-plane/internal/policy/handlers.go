package policy

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

// Handlers exposes HTTP endpoints for managing policies and approvals.
type Handlers struct {
	engine *Engine
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// GetSpacePolicy returns the current policy for a space.
// GET /api/v1/spaces/:id/policy
func (h *Handlers) GetSpacePolicy(c *gin.Context) {
	spaceID := c.Param("id")
	sp := h.engine.GetSpacePolicy(spaceID)
	if sp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no policy configured for space"})
		return
	}
	c.JSON(http.StatusOK, sp)
}

// UpdateSpacePolicyRequest is the payload for PUT /api/v1/spaces/:id/policy.
type UpdateSpacePolicyRequest struct {
	ApprovalMode ApprovalMode `json:"approval_mode" binding:"required"`
	DefaultRules []PolicyRule `json:"default_rules"`
	MaxSpendUSD  float64      `json:"max_spend_usd"`
}

// UpdateSpacePolicy updates the space policy.
// PUT /api/v1/spaces/:id/policy
func (h *Handlers) UpdateSpacePolicy(c *gin.Context) {
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required to update space policy"})
		return
	}

	spaceID := c.Param("id")

	var req UpdateSpacePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidApprovalMode(req.ApprovalMode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval_mode; must be managed, trusted, or bypass"})
		return
	}

	sp := SpacePolicy{
		SpaceID:      spaceID,
		ApprovalMode: req.ApprovalMode,
		DefaultRules: req.DefaultRules,
		MaxSpendUSD:  req.MaxSpendUSD,
	}

	// Preserve created_at if the policy already exists.
	if existing := h.engine.GetSpacePolicy(spaceID); existing != nil {
		sp.CreatedAt = existing.CreatedAt
	}

	h.engine.SetSpacePolicy(sp)
	c.JSON(http.StatusOK, h.engine.GetSpacePolicy(spaceID))
}

// GetBotPolicy returns the policy for a specific bot.
// GET /api/v1/spaces/:id/bots/:botId/policy
func (h *Handlers) GetBotPolicy(c *gin.Context) {
	spaceID := c.Param("id")
	botID := c.Param("bid")

	bp := h.engine.GetBotPolicy(botID, spaceID)
	if bp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no policy configured for bot"})
		return
	}
	c.JSON(http.StatusOK, bp)
}

// UpdateBotPolicyRequest is the payload for PUT /api/v1/spaces/:id/bots/:botId/policy.
type UpdateBotPolicyRequest struct {
	ApprovalMode   ApprovalMode `json:"approval_mode" binding:"required"`
	Rules          []PolicyRule `json:"rules"`
	MaxSpendUSD    float64      `json:"max_spend_usd"`
	AllowedDomains []string     `json:"allowed_domains"`
	DenyPatterns   []string     `json:"deny_patterns"`
}

// UpdateBotPolicy updates a bot's policy.
// PUT /api/v1/spaces/:id/bots/:botId/policy
func (h *Handlers) UpdateBotPolicy(c *gin.Context) {
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required to update bot policy"})
		return
	}

	spaceID := c.Param("id")
	botID := c.Param("bid")

	var req UpdateBotPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidApprovalMode(req.ApprovalMode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval_mode; must be managed, trusted, or bypass"})
		return
	}

	bp := BotPolicy{
		BotID:          botID,
		SpaceID:        spaceID,
		ApprovalMode:   req.ApprovalMode,
		Rules:          req.Rules,
		MaxSpendUSD:    req.MaxSpendUSD,
		AllowedDomains: req.AllowedDomains,
		DenyPatterns:   req.DenyPatterns,
	}

	// Preserve created_at if the policy already exists.
	if existing := h.engine.GetBotPolicy(botID, spaceID); existing != nil {
		bp.CreatedAt = existing.CreatedAt
	}

	h.engine.SetBotPolicy(bp)
	c.JSON(http.StatusOK, h.engine.GetBotPolicy(botID, spaceID))
}

// ToggleBypassRequest is the payload for POST /api/v1/spaces/:id/policy/bypass.
type ToggleBypassRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleBypass toggles bypass mode for a space.
// POST /api/v1/spaces/:id/policy/bypass
func (h *Handlers) ToggleBypass(c *gin.Context) {
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required to toggle bypass mode"})
		return
	}

	spaceID := c.Param("id")

	var req ToggleBypassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.engine.SetBypassMode(spaceID, req.Enabled)

	sp := h.engine.GetSpacePolicy(spaceID)
	c.JSON(http.StatusOK, sp)
}

// ListApprovals returns pending approval requests for a space.
// GET /api/v1/spaces/:id/approvals
func (h *Handlers) ListApprovals(c *gin.Context) {
	spaceID := c.Param("id")
	approvals := h.engine.PendingApprovals(spaceID)
	if approvals == nil {
		approvals = []ApprovalRequest{}
	}
	c.JSON(http.StatusOK, gin.H{"approvals": approvals})
}

// ResolveApprovalRequest is the payload for POST /api/v1/spaces/:id/approvals/:requestId.
type ResolveApprovalRequest struct {
	Approved bool `json:"approved"`
}

// ResolveApproval approves or denies a request.
// POST /api/v1/spaces/:id/approvals/:requestId
func (h *Handlers) ResolveApproval(c *gin.Context) {
	requestID := c.Param("requestId")

	var req ResolveApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// resolvedBy comes from IAM JWT claims. Anonymous approvals are not allowed.
	iamUser := middleware.GetIAMUser(c)
	if iamUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required to resolve approvals"})
		return
	}
	resolvedBy := iamUser.Email
	if resolvedBy == "" {
		resolvedBy = iamUser.Sub
	}

	if err := h.engine.ResolveApproval(requestID, req.Approved, resolvedBy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := "denied"
	if req.Approved {
		status = "approved"
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "request_id": requestID})
}

// SSEApprovals streams approval requests in real-time via Server-Sent Events.
// GET /api/v1/spaces/:id/approvals/stream
func (h *Handlers) SSEApprovals(c *gin.Context) {
	spaceID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	ch := h.engine.ApprovalRequests()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// Send initial keepalive.
	_, _ = io.WriteString(c.Writer, ": keepalive\n\n")
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-ch:
			if !ok {
				return
			}
			// Only forward events for this space.
			if req.SpaceID != spaceID {
				continue
			}
			_, _ = fmt.Fprintf(c.Writer, "event: approval\ndata: {\"id\":%q,\"bot_id\":%q,\"resource\":%q,\"permission\":%q,\"description\":%q}\n\n",
				req.ID, req.BotID, req.Resource, req.Permission, req.Description)
			flusher.Flush()
		case <-ticker.C:
			_, _ = io.WriteString(c.Writer, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func isValidApprovalMode(mode ApprovalMode) bool {
	switch mode {
	case ApprovalManaged, ApprovalTrusted, ApprovalBypass:
		return true
	default:
		return false
	}
}

// isAdmin checks whether the request context indicates an IAM admin.
func isAdmin(c *gin.Context) bool {
	if c.GetString("iam_is_admin") == "true" {
		return true
	}
	if c.GetString("iam_role") == "admin" {
		return true
	}
	return false
}
