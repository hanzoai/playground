package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentEventBus_Subscribe(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	defer unsub()

	require.NotNil(t, ch)
	assert.Equal(t, 1, bus.GetSubscriberCount())
}

func TestAgentEventBus_Unsubscribe(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	assert.Equal(t, 1, bus.GetSubscriberCount())

	unsub()
	assert.Equal(t, 0, bus.GetSubscriberCount())

	// Channel should be closed.
	_, ok := <-ch
	assert.False(t, ok)
}

func TestAgentEventBus_SubscribeAgent(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.SubscribeAgent("space-1", "agent-1")
	defer unsub()

	require.NotNil(t, ch)
	assert.Equal(t, 1, bus.GetSubscriberCount())
}

func TestAgentEventBus_PublishToSpaceSubscriber(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	defer unsub()

	event := AgentEvent{
		Type:      AgentMessage,
		SpaceID:   "space-1",
		AgentID:   "agent-1",
		AgentName: "Bot A",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"content": "hello"},
	}
	bus.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, AgentMessage, received.Type)
		assert.Equal(t, "space-1", received.SpaceID)
		assert.Equal(t, "agent-1", received.AgentID)
		assert.Equal(t, "hello", received.Data["content"])
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}

func TestAgentEventBus_PublishToAgentSubscriber(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.SubscribeAgent("space-1", "agent-1")
	defer unsub()

	// Should receive: same space + same agent
	bus.Publish(AgentEvent{
		Type:    AgentMessage,
		SpaceID: "space-1",
		AgentID: "agent-1",
	})

	select {
	case received := <-ch:
		assert.Equal(t, AgentMessage, received.Type)
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}

	// Should NOT receive: same space but different agent
	bus.Publish(AgentEvent{
		Type:    AgentMessage,
		SpaceID: "space-1",
		AgentID: "agent-2",
	})

	select {
	case <-ch:
		t.Fatal("should not receive event for different agent")
	case <-time.After(100 * time.Millisecond):
		// Expected: no event received.
	}
}

func TestAgentEventBus_SpaceFilteringExcludesOtherSpaces(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	defer unsub()

	// Publish to a different space; subscriber should NOT receive it.
	bus.Publish(AgentEvent{
		Type:    AgentMessage,
		SpaceID: "space-2",
		AgentID: "agent-1",
	})

	select {
	case <-ch:
		t.Fatal("should not receive events from another space")
	case <-time.After(100 * time.Millisecond):
		// Expected.
	}
}

func TestAgentEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewAgentEventBus()
	ch1, unsub1 := bus.Subscribe("space-1")
	ch2, unsub2 := bus.Subscribe("space-1")
	defer unsub1()
	defer unsub2()

	assert.Equal(t, 2, bus.GetSubscriberCount())

	bus.Publish(AgentEvent{
		Type:    AgentTurnStarted,
		SpaceID: "space-1",
		AgentID: "agent-1",
	})

	for _, ch := range []<-chan AgentEvent{ch1, ch2} {
		select {
		case r := <-ch:
			assert.Equal(t, AgentTurnStarted, r.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("event not received by subscriber")
		}
	}
}

func TestAgentEventBus_Concurrent(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	defer unsub()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			bus.Publish(AgentEvent{
				Type:    AgentMessageDelta,
				SpaceID: "space-1",
				AgentID: "agent-1",
			})
		}()
	}
	wg.Wait()

	received := 0
	timeout := time.After(1 * time.Second)
	for received < n {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("only received %d/%d events", received, n)
		}
	}
	assert.Equal(t, n, received)
}

func TestAgentEvent_ToJSON(t *testing.T) {
	event := AgentEvent{
		Type:      AgentToolCallBegin,
		SpaceID:   "space-json",
		AgentID:   "agent-json",
		AgentName: "JSON Bot",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"tool": "grep"},
	}

	str, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, str, "agent.tool.begin")
	assert.Contains(t, str, "space-json")
	assert.Contains(t, str, "agent-json")
	assert.Contains(t, str, "JSON Bot")
}

func TestAgentEventBus_HumanMessageInjected(t *testing.T) {
	bus := NewAgentEventBus()
	ch, unsub := bus.Subscribe("space-1")
	defer unsub()

	bus.Publish(AgentEvent{
		Type:    HumanMessageInjected,
		SpaceID: "space-1",
		AgentID: "agent-1",
		Data:    map[string]interface{}{"message": "stop that", "sender_name": "Alice"},
	})

	select {
	case received := <-ch:
		assert.Equal(t, HumanMessageInjected, received.Type)
		assert.Equal(t, "stop that", received.Data["message"])
		assert.Equal(t, "Alice", received.Data["sender_name"])
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}

func TestGlobalAgentEventBus(t *testing.T) {
	require.NotNil(t, GlobalAgentEventBus)
	ch, unsub := GlobalAgentEventBus.Subscribe("global-test")
	require.NotNil(t, ch)
	unsub()
}

func TestPublishAgentEvent_Helper(t *testing.T) {
	ch, unsub := GlobalAgentEventBus.Subscribe("helper-space")
	defer unsub()

	PublishAgentEvent(AgentJoinedSpace, "helper-space", "a1", "Helper Bot", nil)

	select {
	case evt := <-ch:
		assert.Equal(t, AgentJoinedSpace, evt.Type)
		assert.Equal(t, "helper-space", evt.SpaceID)
		assert.Equal(t, "a1", evt.AgentID)
		assert.Equal(t, "Helper Bot", evt.AgentName)
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}
