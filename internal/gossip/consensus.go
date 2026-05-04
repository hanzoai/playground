package gossip

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	ErrConsensusNotFound = errors.New("consensus request not found")
	ErrAlreadyVoted      = errors.New("agent has already voted on this request")
	ErrNotInSpace        = errors.New("agent is not in the proposal's space")
	ErrConsensusClosed   = errors.New("consensus request is already closed")
	ErrInvalidProposal   = errors.New("proposal text is required")
	ErrInvalidSigs       = errors.New("required_sigs must be > 0")
	ErrInvalidTimeout    = errors.New("timeout must be > 0")
)

// ConsensusRequest describes a proposal that agents in a space vote on.
type ConsensusRequest struct {
	ID           string        `json:"id"`
	SpaceID      string        `json:"space_id"`
	Proposal     string        `json:"proposal"`
	RequiredSigs int           `json:"required_sigs"`
	Timeout      time.Duration `json:"timeout"`
}

// ConsensusResult holds the outcome of a consensus round.
type ConsensusResult struct {
	ID         string    `json:"id"`
	Approved   bool      `json:"approved"`
	Signatures []string  `json:"signatures"` // agent DIDs that approved
	Rejections []string  `json:"rejections"` // agent DIDs that rejected
	Timestamp  time.Time `json:"timestamp"`
}

// pendingConsensus tracks state for an in-progress vote.
type pendingConsensus struct {
	request    ConsensusRequest
	approvals  map[string]struct{} // agentID -> voted yes
	rejections map[string]struct{} // agentID -> voted no
	closed     bool
	result     *ConsensusResult
	done       chan struct{}
}

// Consensus provides lightweight agreement between agents in a space.
// Not full blockchain consensus -- just coordinated voting for
// shared actions (e.g., "all agents agree to deploy").
type Consensus struct {
	tracker *Tracker
	router  *Router

	mu      sync.Mutex
	pending map[string]*pendingConsensus
}

// NewConsensus creates a Consensus coordinator.
func NewConsensus(tracker *Tracker, router *Router) *Consensus {
	return &Consensus{
		tracker: tracker,
		router:  router,
		pending: make(map[string]*pendingConsensus),
	}
}

// Propose initiates a consensus round. It broadcasts the proposal to all agents
// in the space and waits until RequiredSigs approvals are collected or the
// timeout expires.
func (c *Consensus) Propose(ctx context.Context, req ConsensusRequest) (*ConsensusResult, error) {
	if req.ID == "" {
		return nil, ErrEmptyAgentID
	}
	if req.SpaceID == "" {
		return nil, ErrEmptySpaceID
	}
	if req.Proposal == "" {
		return nil, ErrInvalidProposal
	}
	if req.RequiredSigs <= 0 {
		return nil, ErrInvalidSigs
	}
	if req.Timeout <= 0 {
		return nil, ErrInvalidTimeout
	}

	pc := &pendingConsensus{
		request:    req,
		approvals:  make(map[string]struct{}),
		rejections: make(map[string]struct{}),
		done:       make(chan struct{}),
	}

	c.mu.Lock()
	c.pending[req.ID] = pc
	c.mu.Unlock()

	// Broadcast proposal to agents in the space
	payload, _ := json.Marshal(req)
	broadcastMsg := AgentMessage{
		FromAgentID: "consensus:" + req.ID,
		SpaceID:     req.SpaceID,
		MessageType: "consensus",
		Payload:     payload,
	}
	// Best-effort broadcast; agents without subscriptions are skipped.
	_ = c.router.Broadcast(ctx, req.SpaceID, broadcastMsg)

	// Wait for enough votes or timeout
	timer := time.NewTimer(req.Timeout)
	defer timer.Stop()

	select {
	case <-pc.done:
		// Reached required signatures
	case <-timer.C:
		// Timeout: finalize with whatever we have
	case <-ctx.Done():
		// Context cancelled
	}

	c.mu.Lock()
	result := c.finalize(pc)
	c.mu.Unlock()

	return result, nil
}

// Vote records an agent's vote on a pending consensus request.
func (c *Consensus) Vote(ctx context.Context, requestID, agentID string, approve bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	pc, ok := c.pending[requestID]
	if !ok {
		return ErrConsensusNotFound
	}
	if pc.closed {
		return ErrConsensusClosed
	}

	// Check agent voted before
	if _, ok := pc.approvals[agentID]; ok {
		return ErrAlreadyVoted
	}
	if _, ok := pc.rejections[agentID]; ok {
		return ErrAlreadyVoted
	}

	// Verify agent is in the proposal's space
	agent, found := c.tracker.Get(agentID)
	if !found {
		return ErrAgentNotFound
	}
	if agent.SpaceID != pc.request.SpaceID {
		return ErrNotInSpace
	}

	if approve {
		pc.approvals[agentID] = struct{}{}
	} else {
		pc.rejections[agentID] = struct{}{}
	}

	// Check if we reached quorum
	if len(pc.approvals) >= pc.request.RequiredSigs {
		c.finalize(pc)
	}

	return nil
}

// GetResult returns the result of a consensus round, if finalized.
func (c *Consensus) GetResult(requestID string) (*ConsensusResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pc, ok := c.pending[requestID]
	if !ok {
		return nil, ErrConsensusNotFound
	}
	if pc.result == nil {
		return nil, nil // still pending
	}
	return pc.result, nil
}

// finalize closes a pending consensus and records the result.
// Caller must hold c.mu.
func (c *Consensus) finalize(pc *pendingConsensus) *ConsensusResult {
	if pc.result != nil {
		return pc.result
	}

	approvalDIDs := make([]string, 0, len(pc.approvals))
	for agentID := range pc.approvals {
		if agent, ok := c.tracker.Get(agentID); ok {
			approvalDIDs = append(approvalDIDs, agent.DID)
		}
	}

	rejectionDIDs := make([]string, 0, len(pc.rejections))
	for agentID := range pc.rejections {
		if agent, ok := c.tracker.Get(agentID); ok {
			rejectionDIDs = append(rejectionDIDs, agent.DID)
		}
	}

	result := &ConsensusResult{
		ID:         pc.request.ID,
		Approved:   len(pc.approvals) >= pc.request.RequiredSigs,
		Signatures: approvalDIDs,
		Rejections: rejectionDIDs,
		Timestamp:  time.Now(),
	}

	pc.result = result
	pc.closed = true

	// Signal waiters
	select {
	case <-pc.done:
		// already closed
	default:
		close(pc.done)
	}

	return result
}
