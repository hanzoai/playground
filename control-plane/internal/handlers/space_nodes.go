package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// SpaceNodeHandler holds dependencies for node management within a Space.
type SpaceNodeHandler struct {
	store spaces.Store
}

// NewSpaceNodeHandler creates a new SpaceNodeHandler.
func NewSpaceNodeHandler(store spaces.Store) *SpaceNodeHandler {
	return &SpaceNodeHandler{store: store}
}

// RegisterNodeRequest is the payload a hanzo/node sends when self-registering.
type RegisterNodeRequest struct {
	NodeID   string `json:"node_id"`
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type"`     // "local" or "cloud"; defaults to "local"
	Endpoint string `json:"endpoint"` // hanzo/node HTTP API URL
	OS       string `json:"os"`       // "linux", "macos", "windows"
}

// RegisterNode handles POST /api/v1/spaces/:id/nodes/register.
// A hanzo/node calls this on startup to announce itself to a space.
func (h *SpaceNodeHandler) RegisterNode(c *gin.Context) {
	spaceID := c.Param("id")
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = uuid.New().String()[:12]
	}
	nodeType := req.Type
	if nodeType == "" {
		nodeType = "local"
	}

	node := &spaces.SpaceNode{
		SpaceID:      spaceID,
		NodeID:       nodeID,
		Name:         req.Name,
		Type:         nodeType,
		Endpoint:     req.Endpoint,
		Status:       "online",
		OS:           req.OS,
		RegisteredAt: time.Now().UTC(),
		LastSeen:     time.Now().UTC(),
	}

	if err := h.store.RegisterNode(c.Request.Context(), node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

// ListNodes handles GET /api/v1/spaces/:id/nodes.
func (h *SpaceNodeHandler) ListNodes(c *gin.Context) {
	spaceID := c.Param("id")
	nodes, err := h.store.ListNodes(c.Request.Context(), spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nodes == nil {
		nodes = make([]*spaces.SpaceNode, 0)
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// RemoveNode handles DELETE /api/v1/spaces/:id/nodes/:nid.
func (h *SpaceNodeHandler) RemoveNode(c *gin.Context) {
	spaceID := c.Param("id")
	nodeID := c.Param("nid")
	if err := h.store.RemoveNode(c.Request.Context(), spaceID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": true})
}

// Heartbeat handles POST /api/v1/spaces/:id/nodes/:nid/heartbeat.
func (h *SpaceNodeHandler) Heartbeat(c *gin.Context) {
	spaceID := c.Param("id")
	nodeID := c.Param("nid")
	if err := h.store.UpdateNodeStatus(c.Request.Context(), spaceID, nodeID, "online"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
