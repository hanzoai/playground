package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestPresenceHandler() *PresenceHandler {
	bus := events.NewSpaceEventBus()
	h := &PresenceHandler{
		store:    nil, // store not used by presence (in-memory only)
		eventBus: bus,
		stopCh:   make(chan struct{}),
	}
	// Don't start expiryLoop in tests; we'll call expireStale() manually.
	return h
}

func TestCursorUpdate_StoresAndBroadcasts(t *testing.T) {
	h := newTestPresenceHandler()
	defer h.Stop()

	// Subscribe to the global bus to capture the broadcast (drain to avoid blocking).
	_ = events.GlobalSpaceEventBus.Subscribe("test-cursor")
	defer events.GlobalSpaceEventBus.Unsubscribe("test-cursor")

	router := gin.New()
	router.POST("/spaces/:id/presence/cursor", h.CursorUpdate)

	body, _ := json.Marshal(CursorUpdateRequest{X: 10.5, Y: 20.3})
	req := httptest.NewRequest(http.MethodPost, "/spaces/space-1/presence/cursor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])

	// Verify the peer was stored.
	raw, ok := h.peers.Load("space-1")
	require.True(t, ok)
	spaceMap := raw.(*sync.Map)
	peerVal, ok := spaceMap.Load("anonymous")
	require.True(t, ok)
	peer := peerVal.(*PeerPresence)
	assert.Equal(t, 10.5, peer.X)
	assert.Equal(t, 20.3, peer.Y)
	assert.Equal(t, "Anonymous", peer.DisplayName)
}

func TestListPresence_ReturnsPeers(t *testing.T) {
	h := newTestPresenceHandler()
	defer h.Stop()

	// Manually insert peers.
	spaceMap := &sync.Map{}
	spaceMap.Store("user-1", &PeerPresence{
		UserID: "user-1", DisplayName: "Alice", X: 1, Y: 2,
		LastSeen: time.Now().UnixMilli(),
	})
	spaceMap.Store("user-2", &PeerPresence{
		UserID: "user-2", DisplayName: "Bob", X: 3, Y: 4,
		LastSeen: time.Now().UnixMilli(),
	})
	h.peers.Store("space-1", spaceMap)

	router := gin.New()
	router.GET("/spaces/:id/presence", h.ListPresence)

	req := httptest.NewRequest(http.MethodGet, "/spaces/space-1/presence", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Peers []PeerPresence `json:"peers"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Len(t, result.Peers, 2)
}

func TestListPresence_EmptySpace(t *testing.T) {
	h := newTestPresenceHandler()
	defer h.Stop()

	router := gin.New()
	router.GET("/spaces/:id/presence", h.ListPresence)

	req := httptest.NewRequest(http.MethodGet, "/spaces/nonexistent/presence", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Peers []PeerPresence `json:"peers"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Len(t, result.Peers, 0)
}

func TestExpireStale_RemovesOldPeers(t *testing.T) {
	h := newTestPresenceHandler()
	defer h.Stop()

	// Subscribe to catch leave events.
	subCh := events.GlobalSpaceEventBus.Subscribe("test-expire")
	defer events.GlobalSpaceEventBus.Unsubscribe("test-expire")

	spaceMap := &sync.Map{}
	// Stale peer: last seen 60 seconds ago.
	spaceMap.Store("stale-user", &PeerPresence{
		UserID: "stale-user", DisplayName: "Stale",
		X: 0, Y: 0,
		LastSeen: time.Now().Add(-60 * time.Second).UnixMilli(),
	})
	// Fresh peer: last seen 5 seconds ago.
	spaceMap.Store("fresh-user", &PeerPresence{
		UserID: "fresh-user", DisplayName: "Fresh",
		X: 1, Y: 1,
		LastSeen: time.Now().Add(-5 * time.Second).UnixMilli(),
	})
	h.peers.Store("space-1", spaceMap)

	h.expireStale()

	// Stale peer should be gone.
	_, ok := spaceMap.Load("stale-user")
	assert.False(t, ok, "stale peer should be expired")

	// Fresh peer should remain.
	_, ok = spaceMap.Load("fresh-user")
	assert.True(t, ok, "fresh peer should still be present")

	// Drain the leave event.
	select {
	case evt := <-subCh:
		assert.Equal(t, events.SpacePresenceLeave, evt.Type)
		assert.Equal(t, "stale-user", evt.UserID)
	case <-time.After(1 * time.Second):
		t.Fatal("expected leave event for stale peer")
	}
}

func TestCursorUpdate_BadRequest(t *testing.T) {
	h := newTestPresenceHandler()
	defer h.Stop()

	router := gin.New()
	router.POST("/spaces/:id/presence/cursor", h.CursorUpdate)

	// Send invalid JSON.
	req := httptest.NewRequest(http.MethodPost, "/spaces/space-1/presence/cursor",
		bytes.NewReader([]byte(`{"x": "not-a-number"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestPresenceHandler_StopIdempotent(t *testing.T) {
	h := newTestPresenceHandler()
	// Should not panic when called multiple times.
	h.Stop()
	h.Stop()
}
