package gossip

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestAgent(id, space string, caps ...string) AgentInfo {
	capabilities := make([]AgentCapability, len(caps))
	for i, c := range caps {
		capabilities[i] = AgentCapability{Name: c, Description: c + " capability"}
	}
	return AgentInfo{
		AgentID:      id,
		DID:          "did:key:" + id,
		SpaceID:      space,
		DisplayName:  "Agent " + id,
		Capabilities: capabilities,
		Model:        "zen-1",
	}
}

func TestTracker_Register(t *testing.T) {
	tr := NewTracker()
	agent := newTestAgent("a1", "space-1", "code", "search")

	err := tr.Register(agent)
	require.NoError(t, err)
	require.Equal(t, 1, tr.Count())

	got, ok := tr.Get("a1")
	require.True(t, ok)
	require.Equal(t, "a1", got.AgentID)
	require.Equal(t, "online", got.Status) // default status
	require.False(t, got.JoinedAt.IsZero())
}

func TestTracker_Register_Duplicate(t *testing.T) {
	tr := NewTracker()
	agent := newTestAgent("a1", "space-1")

	require.NoError(t, tr.Register(agent))
	err := tr.Register(agent)
	require.ErrorIs(t, err, ErrAgentExists)
}

func TestTracker_Register_Validation(t *testing.T) {
	tr := NewTracker()

	tests := []struct {
		name  string
		agent AgentInfo
		err   error
	}{
		{"empty id", AgentInfo{}, ErrEmptyAgentID},
		{"empty space", AgentInfo{AgentID: "a1"}, ErrEmptySpaceID},
		{"empty did", AgentInfo{AgentID: "a1", SpaceID: "s1"}, ErrEmptyDID},
		{"empty display name", AgentInfo{AgentID: "a1", SpaceID: "s1", DID: "did:key:a1"}, ErrEmptyDisplayName},
		{"invalid status", AgentInfo{AgentID: "a1", SpaceID: "s1", DID: "did:key:a1", DisplayName: "A", Status: "unknown"}, ErrInvalidStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tr.Register(tt.agent)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestTracker_Unregister(t *testing.T) {
	tr := NewTracker()
	agent := newTestAgent("a1", "space-1", "code")

	require.NoError(t, tr.Register(agent))
	require.Equal(t, 1, tr.Count())

	err := tr.Unregister("a1")
	require.NoError(t, err)
	require.Equal(t, 0, tr.Count())

	_, ok := tr.Get("a1")
	require.False(t, ok)

	// Indices should be cleaned up
	require.Empty(t, tr.FindInSpace("space-1"))
	require.Empty(t, tr.FindByCapability("code"))
}

func TestTracker_Unregister_NotFound(t *testing.T) {
	tr := NewTracker()
	err := tr.Unregister("nonexistent")
	require.ErrorIs(t, err, ErrAgentNotFound)
}

func TestTracker_UpdateStatus(t *testing.T) {
	tr := NewTracker()
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	err := tr.UpdateStatus("a1", "busy")
	require.NoError(t, err)

	got, ok := tr.Get("a1")
	require.True(t, ok)
	require.Equal(t, "busy", got.Status)
}

func TestTracker_UpdateStatus_InvalidStatus(t *testing.T) {
	tr := NewTracker()
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	err := tr.UpdateStatus("a1", "sleeping")
	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestTracker_UpdateStatus_NotFound(t *testing.T) {
	tr := NewTracker()
	err := tr.UpdateStatus("nonexistent", "online")
	require.ErrorIs(t, err, ErrAgentNotFound)
}

func TestTracker_FindByCapability(t *testing.T) {
	tr := NewTracker()
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1", "code", "search")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1", "code")))
	require.NoError(t, tr.Register(newTestAgent("a3", "space-1", "deploy")))

	codeAgents := tr.FindByCapability("code")
	require.Len(t, codeAgents, 2)

	searchAgents := tr.FindByCapability("search")
	require.Len(t, searchAgents, 1)

	deployAgents := tr.FindByCapability("deploy")
	require.Len(t, deployAgents, 1)

	noneAgents := tr.FindByCapability("nonexistent")
	require.Empty(t, noneAgents)
}

func TestTracker_FindInSpace(t *testing.T) {
	tr := NewTracker()
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a3", "space-2")))

	s1 := tr.FindInSpace("space-1")
	require.Len(t, s1, 2)

	s2 := tr.FindInSpace("space-2")
	require.Len(t, s2, 1)

	s3 := tr.FindInSpace("space-3")
	require.Empty(t, s3)
}

func TestTracker_All(t *testing.T) {
	tr := NewTracker()
	require.Empty(t, tr.All())

	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-2")))

	all := tr.All()
	require.Len(t, all, 2)
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	tr := NewTracker()
	var wg sync.WaitGroup
	n := 50

	// Concurrent registrations
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("agent-%d", i)
			_ = tr.Register(newTestAgent(id, "space-1", "code"))
		}(i)
	}
	wg.Wait()
	require.Equal(t, n, tr.Count())

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("agent-%d", i)
			_, ok := tr.Get(id)
			require.True(t, ok)
		}(i)
	}
	wg.Wait()

	// Concurrent unregistrations
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("agent-%d", i)
			_ = tr.Unregister(id)
		}(i)
	}
	wg.Wait()
	require.Equal(t, 0, tr.Count())
}

func TestTracker_Unregister_CleansIndices(t *testing.T) {
	tr := NewTracker()
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1", "code")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1", "code")))

	require.NoError(t, tr.Unregister("a1"))

	// "code" capability index should still have a2
	codeAgents := tr.FindByCapability("code")
	require.Len(t, codeAgents, 1)
	require.Equal(t, "a2", codeAgents[0].AgentID)

	// space-1 should still have a2
	spaceAgents := tr.FindInSpace("space-1")
	require.Len(t, spaceAgents, 1)
	require.Equal(t, "a2", spaceAgents[0].AgentID)

	// Remove last agent in space; index entry should be cleaned up
	require.NoError(t, tr.Unregister("a2"))
	require.Empty(t, tr.FindInSpace("space-1"))
	require.Empty(t, tr.FindByCapability("code"))
}
