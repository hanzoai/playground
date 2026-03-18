package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// ChatRoomHandler handles space-wide chat operations.
type ChatRoomHandler struct {
	store spaces.Store
}

// NewChatRoomHandler creates a new ChatRoomHandler.
func NewChatRoomHandler(store spaces.Store) *ChatRoomHandler {
	return &ChatRoomHandler{store: store}
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

// SendMessage receives a chat message from a user and will persist + fan out
// to other connections in the same space. For now it logs and returns the message.
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

	msg := ChatRoomMessage{
		ID:          uuid.New().String()[:12],
		UserID:      userID,
		DisplayName: displayName,
		Text:        req.Message,
		Timestamp:   time.Now().UnixMilli(),
	}

	logger.Logger.Debug().
		Str("space_id", spaceID).
		Str("user_id", userID).
		Str("message_id", msg.ID).
		Msg("chat room message received")

	c.JSON(http.StatusCreated, msg)
}

// GetHistory returns the message history for a space chat room.
// Placeholder: returns an empty list until persistence is implemented.
func (h *ChatRoomHandler) GetHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"messages": []interface{}{}})
}
