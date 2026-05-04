package gossip

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupRouterTest(t *testing.T) (*Tracker, *Router) {
	t.Helper()
	tr := NewTracker()
	r := NewRouter(tr)
	return tr, r
}

func TestRouter_Route(t *testing.T) {
	tr, r := setupRouterTest(t)

	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("receiver", "space-1")))

	ch := r.Subscribe("receiver")

	msg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "receiver",
		SpaceID:     "space-1",
		MessageType: "chat",
		Payload:     []byte("hello"),
	}

	err := r.Route(context.Background(), msg)
	require.NoError(t, err)

	select {
	case received := <-ch:
		require.Equal(t, "sender", received.FromAgentID)
		require.Equal(t, []byte("hello"), received.Payload)
		require.False(t, received.Timestamp.IsZero())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestRouter_Route_SenderNotFound(t *testing.T) {
	tr, r := setupRouterTest(t)
	require.NoError(t, tr.Register(newTestAgent("receiver", "space-1")))
	r.Subscribe("receiver")

	msg := AgentMessage{
		FromAgentID: "nonexistent",
		ToAgentID:   "receiver",
		MessageType: "chat",
	}
	err := r.Route(context.Background(), msg)
	require.ErrorIs(t, err, ErrSenderNotFound)
}

func TestRouter_Route_RecipientNotFound(t *testing.T) {
	tr, r := setupRouterTest(t)
	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))

	msg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "nonexistent",
		MessageType: "chat",
	}
	err := r.Route(context.Background(), msg)
	require.ErrorIs(t, err, ErrRecipientNotFound)
}

func TestRouter_Route_RecipientOffline(t *testing.T) {
	tr, r := setupRouterTest(t)
	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("receiver", "space-1")))
	require.NoError(t, tr.UpdateStatus("receiver", "offline"))
	r.Subscribe("receiver")

	msg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "receiver",
		MessageType: "chat",
	}
	err := r.Route(context.Background(), msg)
	require.ErrorIs(t, err, ErrRecipientOffline)
}

func TestRouter_Route_NoSubscription(t *testing.T) {
	tr, r := setupRouterTest(t)
	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("receiver", "space-1")))

	msg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "receiver",
		MessageType: "chat",
	}
	err := r.Route(context.Background(), msg)
	require.ErrorIs(t, err, ErrNoSubscription)
}

func TestRouter_Route_Validation(t *testing.T) {
	_, r := setupRouterTest(t)

	tests := []struct {
		name string
		msg  AgentMessage
		err  error
	}{
		{"empty from", AgentMessage{}, ErrEmptyFromAgentID},
		{"empty to", AgentMessage{FromAgentID: "a"}, ErrEmptyToAgentID},
		{"empty type", AgentMessage{FromAgentID: "a", ToAgentID: "b"}, ErrEmptyMessageType},
		{"invalid type", AgentMessage{FromAgentID: "a", ToAgentID: "b", MessageType: "invalid"}, ErrInvalidMessageType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Route(context.Background(), tt.msg)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestRouter_Route_ContextCancelled(t *testing.T) {
	tr, r := setupRouterTest(t)
	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("receiver", "space-1")))

	_ = r.Subscribe("receiver")

	// Fill the buffer by routing messages so the next Route will block
	fillMsg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "receiver",
		MessageType: "chat",
	}
	for i := 0; i < defaultChannelBuffer; i++ {
		require.NoError(t, r.Route(context.Background(), fillMsg))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := AgentMessage{
		FromAgentID: "sender",
		ToAgentID:   "receiver",
		MessageType: "chat",
	}
	err := r.Route(ctx, msg)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRouter_Broadcast(t *testing.T) {
	tr, r := setupRouterTest(t)

	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a3", "space-2"))) // different space

	ch1 := r.Subscribe("a1")
	ch2 := r.Subscribe("a2")
	ch3 := r.Subscribe("a3")
	_ = r.Subscribe("sender") // sender subscribes but should not receive own broadcast

	msg := AgentMessage{
		FromAgentID: "sender",
		MessageType: "event",
		Payload:     []byte("deploy"),
	}

	err := r.Broadcast(context.Background(), "space-1", msg)
	require.NoError(t, err)

	// a1 and a2 should receive
	select {
	case m := <-ch1:
		require.Equal(t, "sender", m.FromAgentID)
		require.Equal(t, "space-1", m.SpaceID)
	case <-time.After(time.Second):
		t.Fatal("timeout for a1")
	}

	select {
	case m := <-ch2:
		require.Equal(t, "sender", m.FromAgentID)
	case <-time.After(time.Second):
		t.Fatal("timeout for a2")
	}

	// a3 (different space) should NOT receive
	select {
	case <-ch3:
		t.Fatal("a3 should not receive broadcast from space-1")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestRouter_Broadcast_SkipsOffline(t *testing.T) {
	tr, r := setupRouterTest(t)

	require.NoError(t, tr.Register(newTestAgent("sender", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))
	require.NoError(t, tr.UpdateStatus("a2", "offline"))

	ch1 := r.Subscribe("a1")
	ch2 := r.Subscribe("a2")

	msg := AgentMessage{FromAgentID: "sender", MessageType: "event"}
	err := r.Broadcast(context.Background(), "space-1", msg)
	require.NoError(t, err)

	select {
	case <-ch1:
		// expected
	case <-time.After(time.Second):
		t.Fatal("timeout for a1")
	}

	select {
	case <-ch2:
		t.Fatal("offline agent should not receive broadcast")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestRouter_Subscribe_Resubscribe(t *testing.T) {
	_, r := setupRouterTest(t)

	ch1 := r.Subscribe("a1")
	ch2 := r.Subscribe("a1") // replaces ch1

	// ch1 should be closed
	_, ok := <-ch1
	require.False(t, ok)

	// ch2 should be open
	require.NotNil(t, ch2)
	require.Equal(t, 1, r.SubscriberCount())
}

func TestRouter_Unsubscribe(t *testing.T) {
	_, r := setupRouterTest(t)

	ch := r.Subscribe("a1")
	require.Equal(t, 1, r.SubscriberCount())

	r.Unsubscribe("a1")
	require.Equal(t, 0, r.SubscriberCount())

	// channel should be closed
	_, ok := <-ch
	require.False(t, ok)
}

func TestRouter_Unsubscribe_NotSubscribed(t *testing.T) {
	_, r := setupRouterTest(t)
	// Should not panic
	r.Unsubscribe("nonexistent")
	require.Equal(t, 0, r.SubscriberCount())
}
