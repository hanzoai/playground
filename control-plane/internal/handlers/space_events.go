package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// SpaceEventsHandler provides an SSE endpoint for real-time space events
// (presence updates, chat messages) scoped to a single space.
type SpaceEventsHandler struct {
	eventBus *events.SpaceEventBus
}

// NewSpaceEventsHandler creates a new SpaceEventsHandler.
func NewSpaceEventsHandler(eventBus *events.SpaceEventBus) *SpaceEventsHandler {
	return &SpaceEventsHandler{eventBus: eventBus}
}

// StreamEvents is the SSE handler for GET /api/v1/spaces/:id/events.
// It subscribes to the global SpaceEventBus and forwards only events
// matching the requested spaceID.
func (h *SpaceEventsHandler) StreamEvents(c *gin.Context) {
	spaceID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	subscriberID := fmt.Sprintf("sse_space_%d_%s", time.Now().UnixNano(), spaceID)
	ch := h.eventBus.Subscribe(subscriberID)
	defer h.eventBus.Unsubscribe(subscriberID)

	// Send initial connected event.
	connected := map[string]interface{}{
		"type":     "connected",
		"space_id": spaceID,
		"message":  "Space event stream connected",
	}
	if payload, err := json.Marshal(connected); err == nil {
		writeSpaceSSE(c, payload)
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
				if !writeSpaceSSE(c, payload) {
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				return
			}
			// Only forward events for this space.
			if event.SpaceID != spaceID {
				continue
			}
			payload, err := json.Marshal(event)
			if err != nil {
				logger.Logger.Warn().Err(err).Msg("failed to marshal space event")
				continue
			}
			if !writeSpaceSSE(c, payload) {
				return
			}
		}
	}
}

// writeSpaceSSE writes an SSE data frame and flushes. Returns false if the
// write failed (client disconnected).
func writeSpaceSSE(c *gin.Context, payload []byte) bool {
	if _, err := c.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to write space SSE payload")
		return false
	}
	c.Writer.Flush()
	return true
}
