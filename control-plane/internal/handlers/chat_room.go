package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// ChatRoomHandler handles space-wide chat operations.
type ChatRoomHandler struct {
	store    spaces.Store
	eventBus *events.SpaceEventBus
}

// NewChatRoomHandler creates a new ChatRoomHandler.
func NewChatRoomHandler(store spaces.Store, eventBus *events.SpaceEventBus) *ChatRoomHandler {
	return &ChatRoomHandler{
		store:    store,
		eventBus: eventBus,
	}
}

// ChatRoomSendRequest is the payload for POST /api/v1/spaces/:id/chat.
type ChatRoomSendRequest struct {
	Message string `json:"message" binding:"required"`
}

// ChatRoomMessage is the shape of a chat message returned to clients.
type ChatRoomMessage struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Text        string `json:"text"`
	Timestamp   int64  `json:"timestamp"`
}

// SendMessage persists a chat message to the database and broadcasts it via
// the EventBus so all SSE/WebSocket subscribers in the same space receive it.
func (h *ChatRoomHandler) SendMessage(c *gin.Context) {
	spaceID := c.Param("id")

	var req ChatRoomSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := middleware.GetIAMUser(c)
	userID := "anonymous"
	displayName := "Anonymous"
	if user != nil {
		userID = user.Sub
		if user.Name != "" {
			displayName = user.Name
		}
	}

	now := time.Now().UTC()
	msgID := uuid.New().String()[:12]

	// Persist to database.
	dbMsg := &spaces.ChatMessage{
		ID:          msgID,
		SpaceID:     spaceID,
		UserID:      userID,
		DisplayName: displayName,
		Message:     req.Message,
		CreatedAt:   now,
	}
	if err := h.store.InsertChatMessage(c.Request.Context(), dbMsg); err != nil {
		logger.Logger.Error().Err(err).
			Str("space_id", spaceID).
			Str("user_id", userID).
			Msg("failed to persist chat message")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message"})
		return
	}

	// Build the client-facing response.
	msg := ChatRoomMessage{
		ID:          msgID,
		UserID:      userID,
		DisplayName: displayName,
		Text:        req.Message,
		Timestamp:   now.UnixMilli(),
	}

	// Broadcast via EventBus for real-time delivery.
	events.PublishSpaceEvent(events.SpaceChatMessage, spaceID, userID, msg)

	logger.Logger.Debug().
		Str("space_id", spaceID).
		Str("user_id", userID).
		Str("message_id", msgID).
		Msg("chat room message persisted and broadcast")

	c.JSON(http.StatusCreated, msg)
}

// GetHistory returns the message history for a space chat room.
// Optional query param: ?limit=N (default 50, max 200).
func (h *ChatRoomHandler) GetHistory(c *gin.Context) {
	spaceID := c.Param("id")

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	msgs, err := h.store.ListChatMessages(c.Request.Context(), spaceID, limit)
	if err != nil {
		logger.Logger.Error().Err(err).
			Str("space_id", spaceID).
			Msg("failed to load chat history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}

	// Convert to client-facing shape.
	out := make([]ChatRoomMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ChatRoomMessage{
			ID:          m.ID,
			UserID:      m.UserID,
			DisplayName: m.DisplayName,
			Text:        m.Message,
			Timestamp:   m.CreatedAt.UnixMilli(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"messages": out})
}
