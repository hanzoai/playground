package events

import (
	"encoding/json"
	"sync"
	"time"
)

// BotEventType represents the type of bot event
type BotEventType string

const (
	BotOnline    BotEventType = "bot_online"
	BotOffline   BotEventType = "bot_offline"
	BotUpdated   BotEventType = "bot_updated"
	NodeStatusChanged BotEventType = "node_status_changed"
	BotsRefresh  BotEventType = "bots_refresh"
	Heartbeat         BotEventType = "heartbeat"
)

// BotEvent represents a bot state change event
type BotEvent struct {
	Type       BotEventType `json:"type"`
	BotID string            `json:"bot_id,omitempty"`
	NodeID     string            `json:"node_id,omitempty"`
	Status     string            `json:"status,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	Data       interface{}       `json:"data,omitempty"`
}

// BotEventBus manages bot event broadcasting
type BotEventBus struct {
	subscribers map[string]chan BotEvent
	mutex       sync.RWMutex
}

// NewBotEventBus creates a new bot event bus
func NewBotEventBus() *BotEventBus {
	return &BotEventBus{
		subscribers: make(map[string]chan BotEvent),
	}
}

// Subscribe adds a new subscriber to the event bus
func (bus *BotEventBus) Subscribe(subscriberID string) chan BotEvent {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	ch := make(chan BotEvent, 100) // Buffer to prevent blocking
	bus.subscribers[subscriberID] = ch

	return ch
}

// Unsubscribe removes a subscriber from the event bus
func (bus *BotEventBus) Unsubscribe(subscriberID string) {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	if ch, exists := bus.subscribers[subscriberID]; exists {
		close(ch)
		delete(bus.subscribers, subscriberID)
	}
}

// Publish broadcasts an event to all subscribers
func (bus *BotEventBus) Publish(event BotEvent) {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()

	for _, ch := range bus.subscribers {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Channel is full, skip this subscriber to prevent blocking
		}
	}
}

// GetSubscriberCount returns the number of active subscribers
func (bus *BotEventBus) GetSubscriberCount() int {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()
	return len(bus.subscribers)
}

// ToJSON converts a bot event to JSON string
func (event *BotEvent) ToJSON() (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Global event bus instance
var GlobalBotEventBus = NewBotEventBus()

// Helper functions for common event types

// PublishBotOnline publishes a bot online event
func PublishBotOnline(botID, nodeID string, data interface{}) {
	event := BotEvent{
		Type:       BotOnline,
		BotID: botID,
		NodeID:     nodeID,
		Status:     "online",
		Timestamp:  time.Now(),
		Data:       data,
	}

	GlobalBotEventBus.Publish(event)
}

// PublishBotOffline publishes a bot offline event
func PublishBotOffline(botID, nodeID string, data interface{}) {
	event := BotEvent{
		Type:       BotOffline,
		BotID: botID,
		NodeID:     nodeID,
		Status:     "offline",
		Timestamp:  time.Now(),
		Data:       data,
	}

	GlobalBotEventBus.Publish(event)
}

// PublishBotUpdated publishes a bot updated event
func PublishBotUpdated(botID, nodeID, status string, data interface{}) {
	event := BotEvent{
		Type:       BotUpdated,
		BotID: botID,
		NodeID:     nodeID,
		Status:     status,
		Timestamp:  time.Now(),
		Data:       data,
	}
	GlobalBotEventBus.Publish(event)
}

// PublishNodeStatusChanged publishes a node status change event
func PublishNodeStatusChanged(nodeID, status string, data interface{}) {
	event := BotEvent{
		Type:      NodeStatusChanged,
		NodeID:    nodeID,
		Status:    status,
		Timestamp: time.Now(),
		Data:      data,
	}

	GlobalBotEventBus.Publish(event)
}

// PublishBotsRefresh publishes a general refresh event
func PublishBotsRefresh(data interface{}) {
	event := BotEvent{
		Type:      BotsRefresh,
		Timestamp: time.Now(),
		Data:      data,
	}
	GlobalBotEventBus.Publish(event)
}

// PublishHeartbeat publishes a heartbeat event to keep connections alive
func PublishHeartbeat() {
	event := BotEvent{
		Type:      Heartbeat,
		Timestamp: time.Now(),
	}
	GlobalBotEventBus.Publish(event)
}

// StartHeartbeat starts a goroutine that sends periodic heartbeat events
func StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if GlobalBotEventBus.GetSubscriberCount() > 0 {
				PublishHeartbeat()
			}
		}
	}()
}
