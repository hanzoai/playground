package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// AgentEventType enumerates real-time agent events streamed to the UI.
type AgentEventType string

const (
	AgentTurnStarted     AgentEventType = "agent.turn.started"
	AgentTurnCompleted   AgentEventType = "agent.turn.completed"
	AgentMessage         AgentEventType = "agent.message"
	AgentMessageDelta    AgentEventType = "agent.message.delta"
	AgentExecBegin       AgentEventType = "agent.exec.begin"
	AgentExecEnd         AgentEventType = "agent.exec.end"
	AgentToolCallBegin   AgentEventType = "agent.tool.begin"
	AgentToolCallEnd     AgentEventType = "agent.tool.end"
	AgentStatusChanged   AgentEventType = "agent.status.changed"
	AgentJoinedSpace     AgentEventType = "agent.joined"
	AgentLeftSpace       AgentEventType = "agent.left"
	HumanMessageInjected AgentEventType = "human.message"
)

// AgentEvent is a real-time event from an agent within a space.
type AgentEvent struct {
	Type      AgentEventType         `json:"type"`
	SpaceID   string                 `json:"space_id"`
	AgentID   string                 `json:"agent_id"`
	AgentName string                 `json:"agent_name,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// agentSubscriber represents a subscriber with optional space and agent filters.
type agentSubscriber struct {
	ch      chan AgentEvent
	spaceID string // required filter
	agentID string // optional; empty means all agents in the space
}

// AgentEventBus fans out agent events to SSE subscribers per space.
type AgentEventBus struct {
	subscribers map[string]*agentSubscriber
	mutex       sync.RWMutex
}

// NewAgentEventBus creates a new agent event bus.
func NewAgentEventBus() *AgentEventBus {
	return &AgentEventBus{
		subscribers: make(map[string]*agentSubscriber),
	}
}

// Subscribe registers a subscriber for all agent events in a space.
// Returns a receive-only channel and an unsubscribe function.
func (b *AgentEventBus) Subscribe(spaceID string) (<-chan AgentEvent, func()) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	id := subscriberID("agent_space", spaceID)
	ch := make(chan AgentEvent, 100)
	b.subscribers[id] = &agentSubscriber{
		ch:      ch,
		spaceID: spaceID,
	}

	logger.Logger.Debug().
		Str("subscriber_id", id).
		Str("space_id", spaceID).
		Int("total", len(b.subscribers)).
		Msg("[AgentEventBus] subscriber added")

	return ch, func() { b.unsubscribe(id) }
}

// SubscribeAgent registers a subscriber for events from a specific agent in a space.
// Returns a receive-only channel and an unsubscribe function.
func (b *AgentEventBus) SubscribeAgent(spaceID, agentID string) (<-chan AgentEvent, func()) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	id := subscriberID("agent_single", spaceID+"_"+agentID)
	ch := make(chan AgentEvent, 100)
	b.subscribers[id] = &agentSubscriber{
		ch:      ch,
		spaceID: spaceID,
		agentID: agentID,
	}

	logger.Logger.Debug().
		Str("subscriber_id", id).
		Str("space_id", spaceID).
		Str("agent_id", agentID).
		Int("total", len(b.subscribers)).
		Msg("[AgentEventBus] agent subscriber added")

	return ch, func() { b.unsubscribe(id) }
}

// Publish broadcasts an agent event to all matching subscribers without blocking.
func (b *AgentEventBus) Publish(event AgentEvent) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for id, sub := range b.subscribers {
		if sub.spaceID != event.SpaceID {
			continue
		}
		// AgentID "*" is the broadcast sentinel -- deliver to all subscribers in the space.
		if sub.agentID != "" && event.AgentID != "*" && sub.agentID != event.AgentID {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			logger.Logger.Warn().
				Str("subscriber_id", id).
				Str("event_type", string(event.Type)).
				Msg("[AgentEventBus] channel full, dropping event")
		}
	}
}

// GetSubscriberCount returns the number of active subscribers.
func (b *AgentEventBus) GetSubscriberCount() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.subscribers)
}

// unsubscribe removes a subscriber and closes its channel.
func (b *AgentEventBus) unsubscribe(id string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if sub, ok := b.subscribers[id]; ok {
		close(sub.ch)
		delete(b.subscribers, id)
		logger.Logger.Debug().
			Str("subscriber_id", id).
			Int("total", len(b.subscribers)).
			Msg("[AgentEventBus] subscriber removed")
	}
}

// ToJSON converts an agent event to a JSON string.
func (event *AgentEvent) ToJSON() (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GlobalAgentEventBus is the singleton event bus for all agent real-time events.
var GlobalAgentEventBus = NewAgentEventBus()

// subscriberID generates a unique subscriber ID using a prefix and a key.
func subscriberID(prefix, key string) string {
	return prefix + "_" + time.Now().Format("20060102150405.000000000") + "_" + key
}

// PublishAgentEvent is a convenience wrapper for broadcasting an agent event.
func PublishAgentEvent(eventType AgentEventType, spaceID, agentID, agentName string, data map[string]interface{}) {
	event := AgentEvent{
		Type:      eventType,
		SpaceID:   spaceID,
		AgentID:   agentID,
		AgentName: agentName,
		Timestamp: time.Now(),
		Data:      data,
	}
	GlobalAgentEventBus.Publish(event)
}
