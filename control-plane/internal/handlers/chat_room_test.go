package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChatStore is a minimal spaces.Store that only implements chat methods.
// All other methods return errors.
type mockChatStore struct {
	mu       sync.Mutex
	messages []*spaces.ChatMessage
}

func (m *mockChatStore) InsertChatMessage(_ context.Context, msg *spaces.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatStore) ListChatMessages(_ context.Context, spaceID string, limit int) ([]*spaces.ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*spaces.ChatMessage
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].SpaceID == spaceID {
			result = append(result, m.messages[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// Stub out the rest of spaces.Store so we satisfy the interface.
func (m *mockChatStore) CreateSpace(context.Context, *spaces.Space) error                  { return fmt.Errorf("not implemented") }
func (m *mockChatStore) GetSpace(context.Context, string) (*spaces.Space, error)           { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) ListSpaces(context.Context, string) ([]*spaces.Space, error)       { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) UpdateSpace(context.Context, *spaces.Space) error                  { return fmt.Errorf("not implemented") }
func (m *mockChatStore) DeleteSpace(context.Context, string) error                         { return fmt.Errorf("not implemented") }
func (m *mockChatStore) AddMember(context.Context, *spaces.SpaceMember) error              { return fmt.Errorf("not implemented") }
func (m *mockChatStore) RemoveMember(context.Context, string, string) error                { return fmt.Errorf("not implemented") }
func (m *mockChatStore) ListMembers(context.Context, string) ([]*spaces.SpaceMember, error) { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) GetMember(context.Context, string, string) (*spaces.SpaceMember, error) { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) RegisterNode(context.Context, *spaces.SpaceNode) error             { return fmt.Errorf("not implemented") }
func (m *mockChatStore) GetNode(context.Context, string, string) (*spaces.SpaceNode, error) { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) ListNodes(context.Context, string) ([]*spaces.SpaceNode, error)    { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) UpdateNodeStatus(context.Context, string, string, string) error    { return fmt.Errorf("not implemented") }
func (m *mockChatStore) RemoveNode(context.Context, string, string) error                  { return fmt.Errorf("not implemented") }
func (m *mockChatStore) CreateBot(context.Context, *spaces.SpaceBot) error                 { return fmt.Errorf("not implemented") }
func (m *mockChatStore) GetBot(context.Context, string, string) (*spaces.SpaceBot, error)  { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) ListBots(context.Context, string) ([]*spaces.SpaceBot, error)      { return nil, fmt.Errorf("not implemented") }
func (m *mockChatStore) UpdateBotStatus(context.Context, string, string, string) error     { return fmt.Errorf("not implemented") }
func (m *mockChatStore) RemoveBot(context.Context, string, string) error                   { return fmt.Errorf("not implemented") }
func (m *mockChatStore) Initialize(context.Context) error                                  { return nil }

func TestSendMessage_PersistsAndBroadcasts(t *testing.T) {
	store := &mockChatStore{}
	bus := events.NewSpaceEventBus()
	h := NewChatRoomHandler(store, bus)

	// Subscribe to global bus to catch the broadcast (drain to avoid blocking).
	_ = events.GlobalSpaceEventBus.Subscribe("test-chat-send")
	defer events.GlobalSpaceEventBus.Unsubscribe("test-chat-send")

	router := gin.New()
	router.POST("/spaces/:id/chat", h.SendMessage)

	body, _ := json.Marshal(ChatRoomSendRequest{Message: "Hello, world!"})
	req := httptest.NewRequest(http.MethodPost, "/spaces/space-1/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)

	var msg ChatRoomMessage
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &msg))
	assert.Equal(t, "Hello, world!", msg.Text)
	assert.Equal(t, "anonymous", msg.UserID)
	assert.Equal(t, "Anonymous", msg.DisplayName)
	assert.NotEmpty(t, msg.ID)
	assert.Greater(t, msg.Timestamp, int64(0))

	// Verify persisted.
	store.mu.Lock()
	require.Len(t, store.messages, 1)
	assert.Equal(t, "Hello, world!", store.messages[0].Message)
	assert.Equal(t, "space-1", store.messages[0].SpaceID)
	store.mu.Unlock()
}

func TestSendMessage_BadRequest(t *testing.T) {
	store := &mockChatStore{}
	bus := events.NewSpaceEventBus()
	h := NewChatRoomHandler(store, bus)

	router := gin.New()
	router.POST("/spaces/:id/chat", h.SendMessage)

	// Missing "message" field.
	req := httptest.NewRequest(http.MethodPost, "/spaces/space-1/chat",
		bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetHistory_ReturnsMessages(t *testing.T) {
	store := &mockChatStore{}
	bus := events.NewSpaceEventBus()
	h := NewChatRoomHandler(store, bus)

	// Pre-populate messages.
	for i := 0; i < 5; i++ {
		_ = store.InsertChatMessage(context.Background(), &spaces.ChatMessage{
			ID:          fmt.Sprintf("msg-%d", i),
			SpaceID:     "space-1",
			UserID:      "user-1",
			DisplayName: "Alice",
			Message:     fmt.Sprintf("Message %d", i),
		})
	}

	router := gin.New()
	router.GET("/spaces/:id/chat/history", h.GetHistory)

	req := httptest.NewRequest(http.MethodGet, "/spaces/space-1/chat/history", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Messages []ChatRoomMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Len(t, result.Messages, 5)
}

func TestGetHistory_EmptySpace(t *testing.T) {
	store := &mockChatStore{}
	bus := events.NewSpaceEventBus()
	h := NewChatRoomHandler(store, bus)

	router := gin.New()
	router.GET("/spaces/:id/chat/history", h.GetHistory)

	req := httptest.NewRequest(http.MethodGet, "/spaces/nonexistent/chat/history", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Messages []ChatRoomMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Len(t, result.Messages, 0)
}

func TestGetHistory_WithLimitParam(t *testing.T) {
	store := &mockChatStore{}
	bus := events.NewSpaceEventBus()
	h := NewChatRoomHandler(store, bus)

	// Pre-populate 10 messages.
	for i := 0; i < 10; i++ {
		_ = store.InsertChatMessage(context.Background(), &spaces.ChatMessage{
			ID:          fmt.Sprintf("msg-%d", i),
			SpaceID:     "space-1",
			UserID:      "user-1",
			DisplayName: "Alice",
			Message:     fmt.Sprintf("Message %d", i),
		})
	}

	router := gin.New()
	router.GET("/spaces/:id/chat/history", h.GetHistory)

	req := httptest.NewRequest(http.MethodGet, "/spaces/space-1/chat/history?limit=3", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Messages []ChatRoomMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Len(t, result.Messages, 3)
}
