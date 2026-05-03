package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/hanzoai/playground/internal/logger"
)

// SpaceEventType represents the type of space event.
type SpaceEventType string

const (
	SpaceCursorUpdate SpaceEventType = "presence.cursor.update"
	SpacePresenceJoin SpaceEventType = "presence.join"
	SpacePresenceLeave SpaceEventType = "presence.leave"
	SpaceChatMessage  SpaceEventType = "chat.room.message"
)

// SpaceEvent represents a real-time event within a Space (presence, chat, etc).
type SpaceEvent struct {
	Type      SpaceEventType `json:"type"`
	SpaceID   string         `json:"space_id"`
	UserID    string         `json:"user_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      interface{}    `json:"data,omitempty"`
}

// SpaceEventBus manages per-space event broadcasting.
type SpaceEventBus struct {
	subscribers map[string]chan SpaceEvent
	mutex       sync.RWMutex
}

// NewSpaceEventBus creates a new space event bus.
func NewSpaceEventBus() *SpaceEventBus {
	return &SpaceEventBus{
		subscribers: make(map[string]chan SpaceEvent),
	}
}

// Subscribe registers a subscriber and returns a channel to receive events.
func (bus *SpaceEventBus) Subscribe(subscriberID string) chan SpaceEvent {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	ch := make(chan SpaceEvent, 100)
	bus.subscribers[subscriberID] = ch

	logger.Logger.Debug().
		Str("subscriber_id", subscriberID).
		Int("total", len(bus.subscribers)).
		Msg("[SpaceEventBus] subscriber added")
	return ch
}

// Unsubscribe removes a subscriber and closes the channel.
func (bus *SpaceEventBus) Unsubscribe(subscriberID string) {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	if ch, ok := bus.subscribers[subscriberID]; ok {
		close(ch)
		delete(bus.subscribers, subscriberID)
		logger.Logger.Debug().
			Str("subscriber_id", subscriberID).
			Int("total", len(bus.subscribers)).
			Msg("[SpaceEventBus] subscriber removed")
	}
}

// Publish broadcasts an event to all subscribers without blocking.
func (bus *SpaceEventBus) Publish(event SpaceEvent) {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()

	for id, ch := range bus.subscribers {
		select {
		case ch <- event:
		default:
			logger.Logger.Warn().
				Str("subscriber_id", id).
				Str("event_type", string(event.Type)).
				Msg("[SpaceEventBus] channel full, dropping event")
		}
	}
}

// GetSubscriberCount returns the number of active subscribers.
func (bus *SpaceEventBus) GetSubscriberCount() int {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()
	return len(bus.subscribers)
}

// ToJSON converts a space event to a JSON string.
func (event *SpaceEvent) ToJSON() (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GlobalSpaceEventBus is the singleton event bus for all space real-time events.
var GlobalSpaceEventBus = NewSpaceEventBus()

// PublishSpaceEvent is a convenience wrapper for broadcasting a space event.
func PublishSpaceEvent(eventType SpaceEventType, spaceID, userID string, data interface{}) {
	event := SpaceEvent{
		Type:      eventType,
		SpaceID:   spaceID,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      data,
	}
	GlobalSpaceEventBus.Publish(event)
}
