package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	zappool "github.com/hanzoai/playground/control-plane/internal/zap"
)

// SpaceAgentEventsHandler provides SSE endpoints for streaming agent events to the browser.
type SpaceAgentEventsHandler struct {
	eventBus *events.AgentEventBus
	zapPool  *zappool.Pool
}

// NewSpaceAgentEventsHandler creates a new SpaceAgentEventsHandler.
func NewSpaceAgentEventsHandler(eventBus *events.AgentEventBus, zapPool *zappool.Pool) *SpaceAgentEventsHandler {
	return &SpaceAgentEventsHandler{eventBus: eventBus, zapPool: zapPool}
}

// HandleSSE streams all agent events for a space via Server-Sent Events.
// GET /api/v1/spaces/:id/agents/events
func (h *SpaceAgentEventsHandler) HandleSSE(c *gin.Context) {
	spaceID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	ch, unsub := h.eventBus.Subscribe(spaceID)
	defer unsub()

	// Send initial connected event.
	connected := map[string]interface{}{
		"type":     "connected",
		"space_id": spaceID,
		"message":  "Agent event stream connected",
	}
	if h.zapPool != nil {
		connected["sidecar_count"] = len(h.zapPool.ForSpace(spaceID))
	}
	if payload, err := json.Marshal(connected); err == nil {
		writeAgentSSE(c, payload)
	}

	ctx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			if h.zapPool != nil {
				heartbeat["sidecar_count"] = len(h.zapPool.ForSpace(spaceID))
			}
			if payload, err := json.Marshal(heartbeat); err == nil {
				if !writeAgentSSE(c, payload) {
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				logger.Logger.Warn().Err(err).Msg("failed to marshal agent event")
				continue
			}
			if !writeAgentSSE(c, payload) {
				return
			}
		}
	}
}

// HandleAgentSSE streams events for a specific agent in a space.
// GET /api/v1/spaces/:id/agents/:agentId/events
func (h *SpaceAgentEventsHandler) HandleAgentSSE(c *gin.Context) {
	spaceID := c.Param("id")
	agentID := c.Param("agentId")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	ch, unsub := h.eventBus.SubscribeAgent(spaceID, agentID)
	defer unsub()

	// Send initial connected event.
	connected := map[string]interface{}{
		"type":     "connected",
		"space_id": spaceID,
		"agent_id": agentID,
		"message":  fmt.Sprintf("Agent %s event stream connected", agentID),
	}
	if h.zapPool != nil {
		_, hasSidecar := h.zapPool.Get(agentID)
		connected["sidecar_connected"] = hasSidecar
	}
	if payload, err := json.Marshal(connected); err == nil {
		writeAgentSSE(c, payload)
	}

	ctx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			if payload, err := json.Marshal(heartbeat); err == nil {
				if !writeAgentSSE(c, payload) {
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				logger.Logger.Warn().Err(err).Msg("failed to marshal agent event")
				continue
			}
			if !writeAgentSSE(c, payload) {
				return
			}
		}
	}
}

// writeAgentSSE writes an SSE data frame and flushes. Returns false if the
// write failed (client disconnected).
func writeAgentSSE(c *gin.Context, payload []byte) bool {
	if _, err := c.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to write agent SSE payload")
		return false
	}
	c.Writer.Flush()
	return true
}
