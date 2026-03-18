package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// PresenceHandler handles real-time presence operations within a Space.
type PresenceHandler struct {
	store spaces.Store
}

// NewPresenceHandler creates a new PresenceHandler.
func NewPresenceHandler(store spaces.Store) *PresenceHandler {
	return &PresenceHandler{store: store}
}

// CursorUpdateRequest is the payload for POST /api/v1/spaces/:id/presence/cursor.
type CursorUpdateRequest struct {
	X float64 `json:"x" binding:"required"`
	Y float64 `json:"y" binding:"required"`
}

// CursorUpdate receives a cursor position broadcast from a client.
// In the next phase this will fan out to other connections in the same space
// via WebSocket. For now it logs and returns success.
func (h *PresenceHandler) CursorUpdate(c *gin.Context) {
	spaceID := c.Param("id")

	var req CursorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Logger.Debug().
		Str("space_id", spaceID).
		Float64("x", req.X).
		Float64("y", req.Y).
		Msg("presence cursor update")

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListPresence returns the currently-known peers for a space.
// Placeholder: returns an empty list until full presence tracking is implemented.
func (h *PresenceHandler) ListPresence(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"peers": []interface{}{}})
}
