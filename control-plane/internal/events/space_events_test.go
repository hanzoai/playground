package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpaceEventBus_Subscribe(t *testing.T) {
	bus := NewSpaceEventBus()
	ch := bus.Subscribe("sub-1")
	require.NotNil(t, ch)
	assert.Equal(t, 1, bus.GetSubscriberCount())
}

func TestSpaceEventBus_Unsubscribe(t *testing.T) {
	bus := NewSpaceEventBus()
	ch := bus.Subscribe("sub-1")
	assert.Equal(t, 1, bus.GetSubscriberCount())

	bus.Unsubscribe("sub-1")
	assert.Equal(t, 0, bus.GetSubscriberCount())

	// Channel should be closed.
	_, ok := <-ch
	assert.False(t, ok)
}

func TestSpaceEventBus_Publish(t *testing.T) {
	bus := NewSpaceEventBus()
	ch := bus.Subscribe("sub-1")
	defer bus.Unsubscribe("sub-1")

	event := SpaceEvent{
		Type:    SpaceCursorUpdate,
		SpaceID: "space-1",
		UserID:  "user-1",
	}
	bus.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, SpaceCursorUpdate, received.Type)
		assert.Equal(t, "space-1", received.SpaceID)
		assert.Equal(t, "user-1", received.UserID)
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}

func TestSpaceEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewSpaceEventBus()
	ch1 := bus.Subscribe("sub-1")
	ch2 := bus.Subscribe("sub-2")
	defer bus.Unsubscribe("sub-1")
	defer bus.Unsubscribe("sub-2")

	event := SpaceEvent{Type: SpaceChatMessage, SpaceID: "s1"}
	bus.Publish(event)

	for _, ch := range []chan SpaceEvent{ch1, ch2} {
		select {
		case r := <-ch:
			assert.Equal(t, SpaceChatMessage, r.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("event not received")
		}
	}
}

func TestSpaceEventBus_Concurrent(t *testing.T) {
	bus := NewSpaceEventBus()
	ch := bus.Subscribe("sub-1")
	defer bus.Unsubscribe("sub-1")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			bus.Publish(SpaceEvent{Type: SpaceCursorUpdate, SpaceID: "s1"})
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

func TestSpaceEvent_ToJSON(t *testing.T) {
	event := SpaceEvent{
		Type:      SpaceChatMessage,
		SpaceID:   "space-json",
		UserID:    "user-1",
		Timestamp: time.Now(),
		Data:      map[string]string{"text": "hello"},
	}
	str, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, str, "space-json")
	assert.Contains(t, str, "chat.room.message")
}

func TestGlobalSpaceEventBus(t *testing.T) {
	require.NotNil(t, GlobalSpaceEventBus)
	ch := GlobalSpaceEventBus.Subscribe("global-test")
	require.NotNil(t, ch)
	GlobalSpaceEventBus.Unsubscribe("global-test")
}

func TestPublishSpaceEvent_Helper(t *testing.T) {
	ch := GlobalSpaceEventBus.Subscribe("helper-test")
	defer GlobalSpaceEventBus.Unsubscribe("helper-test")

	PublishSpaceEvent(SpacePresenceJoin, "s1", "u1", nil)

	select {
	case evt := <-ch:
		assert.Equal(t, SpacePresenceJoin, evt.Type)
		assert.Equal(t, "s1", evt.SpaceID)
		assert.Equal(t, "u1", evt.UserID)
	case <-time.After(1 * time.Second):
		t.Fatal("event not received")
	}
}
