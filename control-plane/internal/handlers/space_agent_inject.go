package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/events"
)

// SpaceAgentInjectHandler lets humans inject messages into agent conversations.
type SpaceAgentInjectHandler struct {
	eventBus *events.AgentEventBus
}

// NewSpaceAgentInjectHandler creates a new SpaceAgentInjectHandler.
func NewSpaceAgentInjectHandler(eventBus *events.AgentEventBus) *SpaceAgentInjectHandler {
	return &SpaceAgentInjectHandler{eventBus: eventBus}
}

// injectRequest is the JSON body for injecting a human message.
type injectRequest struct {
	Message    string `json:"message" binding:"required"`
	SenderName string `json:"sender_name" binding:"required"`
}

// InjectMessage allows a human to send a message into an agent's conversation.
// POST /api/v1/spaces/:id/agents/:agentId/inject
func (h *SpaceAgentInjectHandler) InjectMessage(c *gin.Context) {
	spaceID := c.Param("id")
	agentID := c.Param("agentId")

	var req injectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := events.AgentEvent{
		Type:    events.HumanMessageInjected,
		SpaceID: spaceID,
		AgentID: agentID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message":     req.Message,
			"sender_name": req.SenderName,
		},
	}

	h.eventBus.Publish(event)

	c.JSON(http.StatusOK, gin.H{
		"status":   "delivered",
		"space_id": spaceID,
		"agent_id": agentID,
	})
}

// BroadcastMessage sends a human message to all agents in a space.
// POST /api/v1/spaces/:id/agents/broadcast
func (h *SpaceAgentInjectHandler) BroadcastMessage(c *gin.Context) {
	spaceID := c.Param("id")

	var req injectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := events.AgentEvent{
		Type:    events.HumanMessageInjected,
		SpaceID: spaceID,
		AgentID: "*", // broadcast sentinel
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message":     req.Message,
			"sender_name": req.SenderName,
			"broadcast":   true,
		},
	}

	h.eventBus.Publish(event)

	c.JSON(http.StatusOK, gin.H{
		"status":   "broadcast",
		"space_id": spaceID,
	})
}
