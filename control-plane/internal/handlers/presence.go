package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// PeerPresence tracks a single user's cursor position and metadata.
type PeerPresence struct {
	UserID      string  `json:"userId"`
	DisplayName string  `json:"displayName"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	LastSeen    int64   `json:"lastSeen"`
}

// PresenceHandler handles real-time presence operations within a Space.
type PresenceHandler struct {
	store    spaces.Store
	eventBus *events.SpaceEventBus

	// peers maps spaceID -> (userID -> *PeerPresence).
	// The outer sync.Map avoids a global lock on unrelated spaces.
	peers sync.Map

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewPresenceHandler creates a new PresenceHandler and starts the expiry goroutine.
func NewPresenceHandler(store spaces.Store, eventBus *events.SpaceEventBus) *PresenceHandler {
	h := &PresenceHandler{
		store:    store,
		eventBus: eventBus,
		stopCh:   make(chan struct{}),
	}
	go h.expiryLoop()
	return h
}

// Stop terminates the background expiry goroutine.
func (h *PresenceHandler) Stop() {
	h.stopOnce.Do(func() { close(h.stopCh) })
}

// CursorUpdateRequest is the payload for POST /api/v1/spaces/:id/presence/cursor.
type CursorUpdateRequest struct {
	X float64 `json:"x" binding:"required"`
	Y float64 `json:"y" binding:"required"`
}

// CursorUpdate receives a cursor position from a client, stores it in the
// in-memory map, and broadcasts to all EventBus subscribers.
func (h *PresenceHandler) CursorUpdate(c *gin.Context) {
	spaceID := c.Param("id")

	var req CursorUpdateRequest
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

	peer := &PeerPresence{
		UserID:      userID,
		DisplayName: displayName,
		X:           req.X,
		Y:           req.Y,
		LastSeen:    time.Now().UnixMilli(),
	}

	// Upsert into the per-space map.
	spaceMap := h.getOrCreateSpaceMap(spaceID)

	_, alreadyPresent := spaceMap.Load(userID)
	spaceMap.Store(userID, peer)

	// Broadcast cursor update via EventBus.
	events.PublishSpaceEvent(events.SpaceCursorUpdate, spaceID, userID, peer)

	// If this is a new peer joining, also emit a join event.
	if !alreadyPresent {
		events.PublishSpaceEvent(events.SpacePresenceJoin, spaceID, userID, peer)
		logger.Logger.Debug().
			Str("space_id", spaceID).
			Str("user_id", userID).
			Msg("presence peer joined")
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListPresence returns the currently-known peers for a space.
func (h *PresenceHandler) ListPresence(c *gin.Context) {
	spaceID := c.Param("id")

	var peers []*PeerPresence
	if raw, ok := h.peers.Load(spaceID); ok {
		spaceMap := raw.(*sync.Map)
		spaceMap.Range(func(_, v interface{}) bool {
			peers = append(peers, v.(*PeerPresence))
			return true
		})
	}

	if peers == nil {
		peers = make([]*PeerPresence, 0)
	}

	c.JSON(http.StatusOK, gin.H{"peers": peers})
}

// getOrCreateSpaceMap returns the sync.Map for the given space, creating one if needed.
func (h *PresenceHandler) getOrCreateSpaceMap(spaceID string) *sync.Map {
	actual, _ := h.peers.LoadOrStore(spaceID, &sync.Map{})
	return actual.(*sync.Map)
}

// expiryLoop runs every 10 seconds and removes peers that haven't sent an
// update in 30 seconds.
func (h *PresenceHandler) expiryLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.expireStale()
		}
	}
}

// expireStale removes peers that haven't updated in 30 seconds and emits
// leave events for them.
func (h *PresenceHandler) expireStale() {
	cutoff := time.Now().Add(-30 * time.Second).UnixMilli()

	h.peers.Range(func(spaceKey, spaceVal interface{}) bool {
		spaceID := spaceKey.(string)
		spaceMap := spaceVal.(*sync.Map)

		spaceMap.Range(func(userKey, peerVal interface{}) bool {
			peer := peerVal.(*PeerPresence)
			if peer.LastSeen < cutoff {
				spaceMap.Delete(userKey)
				events.PublishSpaceEvent(events.SpacePresenceLeave, spaceID, peer.UserID, peer)
				logger.Logger.Debug().
					Str("space_id", spaceID).
					Str("user_id", peer.UserID).
					Msg("presence peer expired")
			}
			return true
		})
		return true
	})
}
