package gossip

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrRecipientNotFound  = errors.New("recipient agent not found")
	ErrRecipientOffline   = errors.New("recipient agent is offline")
	ErrNoSubscription     = errors.New("recipient has no active subscription")
	ErrSenderNotFound     = errors.New("sender agent not found")
	ErrEmptyFromAgentID   = errors.New("from_agent_id is required")
	ErrEmptyToAgentID     = errors.New("to_agent_id is required")
	ErrEmptyMessageType   = errors.New("message_type is required")
	ErrInvalidMessageType = errors.New("invalid message_type: must be chat, task, event, or consensus")
)

var validMessageTypes = map[string]bool{
	"chat":      true,
	"task":      true,
	"event":     true,
	"consensus": true,
}

// AgentMessage is a message routed between agents.
type AgentMessage struct {
	FromAgentID string    `json:"from_agent_id"`
	ToAgentID   string    `json:"to_agent_id"`
	SpaceID     string    `json:"space_id"`
	MessageType string    `json:"message_type"` // "chat", "task", "event", "consensus"
	Payload     []byte    `json:"payload"`
	Timestamp   time.Time `json:"timestamp"`
}

const defaultChannelBuffer = 100

// Router delivers messages between agents using the Tracker for discovery.
type Router struct {
	tracker *Tracker

	mu          sync.RWMutex
	subscribers map[string]chan AgentMessage // agentID -> message channel
}

// NewRouter creates a Router backed by the given Tracker.
func NewRouter(tracker *Tracker) *Router {
	return &Router{
		tracker:     tracker,
		subscribers: make(map[string]chan AgentMessage),
	}
}

// Route sends a message to a single agent. The recipient must be registered
// and have an active subscription.
func (r *Router) Route(ctx context.Context, msg AgentMessage) error {
	if err := validateMessage(msg); err != nil {
		return err
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Verify sender exists
	if _, ok := r.tracker.Get(msg.FromAgentID); !ok {
		return ErrSenderNotFound
	}

	// Verify recipient exists and is not offline
	recipient, ok := r.tracker.Get(msg.ToAgentID)
	if !ok {
		return ErrRecipientNotFound
	}
	if recipient.Status == "offline" {
		return ErrRecipientOffline
	}

	r.mu.RLock()
	ch, ok := r.subscribers[msg.ToAgentID]
	r.mu.RUnlock()

	if !ok {
		return ErrNoSubscription
	}

	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Broadcast sends a message to all subscribed agents in the given space,
// excluding the sender.
func (r *Router) Broadcast(ctx context.Context, spaceID string, msg AgentMessage) error {
	if msg.FromAgentID == "" {
		return ErrEmptyFromAgentID
	}
	if msg.MessageType == "" {
		return ErrEmptyMessageType
	}
	if !validMessageTypes[msg.MessageType] {
		return ErrInvalidMessageType
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	msg.SpaceID = spaceID

	agents := r.tracker.FindInSpace(spaceID)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range agents {
		if agent.AgentID == msg.FromAgentID {
			continue
		}
		if agent.Status == "offline" {
			continue
		}

		ch, ok := r.subscribers[agent.AgentID]
		if !ok {
			continue
		}

		// Non-blocking send: drop messages for slow subscribers
		select {
		case ch <- msg:
		default:
		}
	}

	return nil
}

// Subscribe registers an agent to receive messages. Returns a read channel.
func (r *Router) Subscribe(agentID string) <-chan AgentMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close existing subscription if present
	if existing, ok := r.subscribers[agentID]; ok {
		close(existing)
	}

	ch := make(chan AgentMessage, defaultChannelBuffer)
	r.subscribers[agentID] = ch
	return ch
}

// Unsubscribe removes an agent's subscription and closes the channel.
func (r *Router) Unsubscribe(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ch, ok := r.subscribers[agentID]; ok {
		close(ch)
		delete(r.subscribers, agentID)
	}
}

// SubscriberCount returns the number of active subscriptions.
func (r *Router) SubscriberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subscribers)
}

func validateMessage(msg AgentMessage) error {
	if msg.FromAgentID == "" {
		return ErrEmptyFromAgentID
	}
	if msg.ToAgentID == "" {
		return ErrEmptyToAgentID
	}
	if msg.MessageType == "" {
		return ErrEmptyMessageType
	}
	if !validMessageTypes[msg.MessageType] {
		return ErrInvalidMessageType
	}
	return nil
}
