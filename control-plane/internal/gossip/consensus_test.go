package gossip

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupConsensusTest(t *testing.T) (*Tracker, *Router, *Consensus) {
	t.Helper()
	tr := NewTracker()
	r := NewRouter(tr)
	c := NewConsensus(tr, r)
	return tr, r, c
}

func TestConsensus_Propose_Approved(t *testing.T) {
	tr, _, c := setupConsensusTest(t)

	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a3", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-1",
		SpaceID:      "space-1",
		Proposal:     "deploy v2",
		RequiredSigs: 2,
		Timeout:      2 * time.Second,
	}

	// Vote in a goroutine before Propose returns
	go func() {
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, c.Vote(context.Background(), "proposal-1", "a1", true))
		require.NoError(t, c.Vote(context.Background(), "proposal-1", "a2", true))
	}()

	result, err := c.Propose(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.Approved)
	require.Len(t, result.Signatures, 2)
	require.Empty(t, result.Rejections)
	require.False(t, result.Timestamp.IsZero())
}

func TestConsensus_Propose_Rejected(t *testing.T) {
	tr, _, c := setupConsensusTest(t)

	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-2",
		SpaceID:      "space-1",
		Proposal:     "deploy v2",
		RequiredSigs: 2,
		Timeout:      200 * time.Millisecond,
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, c.Vote(context.Background(), "proposal-2", "a1", true))
		require.NoError(t, c.Vote(context.Background(), "proposal-2", "a2", false))
	}()

	result, err := c.Propose(context.Background(), req)
	require.NoError(t, err)
	require.False(t, result.Approved) // only 1 approval, needed 2
	require.Len(t, result.Signatures, 1)
	require.Len(t, result.Rejections, 1)
}

func TestConsensus_Propose_Timeout(t *testing.T) {
	tr, _, c := setupConsensusTest(t)

	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-3",
		SpaceID:      "space-1",
		Proposal:     "deploy v2",
		RequiredSigs: 3,
		Timeout:      100 * time.Millisecond,
	}

	// Only one vote, need 3
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = c.Vote(context.Background(), "proposal-3", "a1", true)
	}()

	result, err := c.Propose(context.Background(), req)
	require.NoError(t, err)
	require.False(t, result.Approved)
}

func TestConsensus_Vote_DoubleVote(t *testing.T) {
	tr, _, c := setupConsensusTest(t)
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-4",
		SpaceID:      "space-1",
		Proposal:     "test",
		RequiredSigs: 5,
		Timeout:      2 * time.Second,
	}

	// Start proposal in background (it will wait for timeout)
	go func() {
		_, _ = c.Propose(context.Background(), req)
	}()
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, c.Vote(context.Background(), "proposal-4", "a1", true))
	err := c.Vote(context.Background(), "proposal-4", "a1", true)
	require.ErrorIs(t, err, ErrAlreadyVoted)
}

func TestConsensus_Vote_NotInSpace(t *testing.T) {
	tr, _, c := setupConsensusTest(t)
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-2"))) // different space

	req := ConsensusRequest{
		ID:           "proposal-5",
		SpaceID:      "space-1",
		Proposal:     "test",
		RequiredSigs: 5,
		Timeout:      2 * time.Second,
	}

	go func() {
		_, _ = c.Propose(context.Background(), req)
	}()
	time.Sleep(50 * time.Millisecond)

	err := c.Vote(context.Background(), "proposal-5", "a2", true)
	require.ErrorIs(t, err, ErrNotInSpace)
}

func TestConsensus_Vote_NotFound(t *testing.T) {
	_, _, c := setupConsensusTest(t)
	err := c.Vote(context.Background(), "nonexistent", "a1", true)
	require.ErrorIs(t, err, ErrConsensusNotFound)
}

func TestConsensus_Vote_AgentNotFound(t *testing.T) {
	tr, _, c := setupConsensusTest(t)
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-6",
		SpaceID:      "space-1",
		Proposal:     "test",
		RequiredSigs: 5,
		Timeout:      2 * time.Second,
	}

	go func() {
		_, _ = c.Propose(context.Background(), req)
	}()
	time.Sleep(50 * time.Millisecond)

	err := c.Vote(context.Background(), "proposal-6", "ghost", true)
	require.ErrorIs(t, err, ErrAgentNotFound)
}

func TestConsensus_Propose_Validation(t *testing.T) {
	_, _, c := setupConsensusTest(t)

	tests := []struct {
		name string
		req  ConsensusRequest
		err  error
	}{
		{"empty id", ConsensusRequest{}, ErrEmptyAgentID},
		{"empty space", ConsensusRequest{ID: "p1"}, ErrEmptySpaceID},
		{"empty proposal", ConsensusRequest{ID: "p1", SpaceID: "s1"}, ErrInvalidProposal},
		{"zero sigs", ConsensusRequest{ID: "p1", SpaceID: "s1", Proposal: "x", RequiredSigs: 0}, ErrInvalidSigs},
		{"zero timeout", ConsensusRequest{ID: "p1", SpaceID: "s1", Proposal: "x", RequiredSigs: 1}, ErrInvalidTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Propose(context.Background(), tt.req)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestConsensus_GetResult(t *testing.T) {
	tr, _, c := setupConsensusTest(t)
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-7",
		SpaceID:      "space-1",
		Proposal:     "test",
		RequiredSigs: 1,
		Timeout:      2 * time.Second,
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = c.Vote(context.Background(), "proposal-7", "a1", true)
	}()

	_, err := c.Propose(context.Background(), req)
	require.NoError(t, err)

	result, err := c.GetResult("proposal-7")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Approved)
}

func TestConsensus_GetResult_NotFound(t *testing.T) {
	_, _, c := setupConsensusTest(t)
	_, err := c.GetResult("nonexistent")
	require.ErrorIs(t, err, ErrConsensusNotFound)
}

func TestConsensus_Vote_AfterClosed(t *testing.T) {
	tr, _, c := setupConsensusTest(t)
	require.NoError(t, tr.Register(newTestAgent("a1", "space-1")))
	require.NoError(t, tr.Register(newTestAgent("a2", "space-1")))

	req := ConsensusRequest{
		ID:           "proposal-8",
		SpaceID:      "space-1",
		Proposal:     "test",
		RequiredSigs: 1,
		Timeout:      2 * time.Second,
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = c.Vote(context.Background(), "proposal-8", "a1", true) // this closes it
	}()

	_, err := c.Propose(context.Background(), req)
	require.NoError(t, err)

	// Voting after consensus closed should fail
	err = c.Vote(context.Background(), "proposal-8", "a2", true)
	require.ErrorIs(t, err, ErrConsensusClosed)
}
