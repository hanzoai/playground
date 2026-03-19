package gossip

import (
	"errors"
	"sync"
	"time"
)

// AgentCapability represents a skill or function an agent can perform.
type AgentCapability struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// AgentInfo holds discoverable metadata about an agent.
type AgentInfo struct {
	AgentID      string            `json:"agent_id"`
	DID          string            `json:"did"`
	SpaceID      string            `json:"space_id"`
	DisplayName  string            `json:"display_name"`
	Status       string            `json:"status"` // "online", "offline", "busy"
	Capabilities []AgentCapability `json:"capabilities"`
	Model        string            `json:"model"`
	JoinedAt     time.Time         `json:"joined_at"`
}

var validStatuses = map[string]bool{
	"online":  true,
	"offline": true,
	"busy":    true,
}

var (
	ErrAgentNotFound    = errors.New("agent not found")
	ErrAgentExists      = errors.New("agent already registered")
	ErrInvalidStatus    = errors.New("invalid status: must be online, offline, or busy")
	ErrEmptyAgentID     = errors.New("agent_id is required")
	ErrEmptySpaceID     = errors.New("space_id is required")
	ErrEmptyDID         = errors.New("did is required")
	ErrEmptyDisplayName = errors.New("display_name is required")
)

// Tracker manages agent discovery within and across spaces.
// Inspired by Lux GossipTracker's bitset-based peer knowledge tracking,
// adapted for agent capability discovery instead of validator gossip.
type Tracker struct {
	mu sync.RWMutex

	// agents indexed by agent ID for O(1) lookup
	agents map[string]AgentInfo

	// secondary indices for efficient queries
	bySpace      map[string]map[string]struct{} // spaceID -> set of agentIDs
	byCapability map[string]map[string]struct{} // capability name -> set of agentIDs
}

// NewTracker creates a Tracker ready for use.
func NewTracker() *Tracker {
	return &Tracker{
		agents:       make(map[string]AgentInfo),
		bySpace:      make(map[string]map[string]struct{}),
		byCapability: make(map[string]map[string]struct{}),
	}
}

// Register adds an agent to the tracker. Returns ErrAgentExists if already registered.
func (t *Tracker) Register(agent AgentInfo) error {
	if agent.AgentID == "" {
		return ErrEmptyAgentID
	}
	if agent.SpaceID == "" {
		return ErrEmptySpaceID
	}
	if agent.DID == "" {
		return ErrEmptyDID
	}
	if agent.DisplayName == "" {
		return ErrEmptyDisplayName
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.agents[agent.AgentID]; exists {
		return ErrAgentExists
	}

	if agent.Status == "" {
		agent.Status = "online"
	}
	if !validStatuses[agent.Status] {
		return ErrInvalidStatus
	}
	if agent.JoinedAt.IsZero() {
		agent.JoinedAt = time.Now()
	}

	t.agents[agent.AgentID] = agent

	// index by space
	if t.bySpace[agent.SpaceID] == nil {
		t.bySpace[agent.SpaceID] = make(map[string]struct{})
	}
	t.bySpace[agent.SpaceID][agent.AgentID] = struct{}{}

	// index by capabilities
	for _, cap := range agent.Capabilities {
		if t.byCapability[cap.Name] == nil {
			t.byCapability[cap.Name] = make(map[string]struct{})
		}
		t.byCapability[cap.Name][agent.AgentID] = struct{}{}
	}

	return nil
}

// Unregister removes an agent from the tracker. Returns ErrAgentNotFound if not present.
func (t *Tracker) Unregister(agentID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	agent, exists := t.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}

	// remove from space index
	if spaceAgents, ok := t.bySpace[agent.SpaceID]; ok {
		delete(spaceAgents, agentID)
		if len(spaceAgents) == 0 {
			delete(t.bySpace, agent.SpaceID)
		}
	}

	// remove from capability indices
	for _, cap := range agent.Capabilities {
		if capAgents, ok := t.byCapability[cap.Name]; ok {
			delete(capAgents, agentID)
			if len(capAgents) == 0 {
				delete(t.byCapability, cap.Name)
			}
		}
	}

	delete(t.agents, agentID)
	return nil
}

// UpdateStatus changes an agent's status. Returns ErrAgentNotFound or ErrInvalidStatus.
func (t *Tracker) UpdateStatus(agentID, status string) error {
	if !validStatuses[status] {
		return ErrInvalidStatus
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	agent, exists := t.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}

	agent.Status = status
	t.agents[agentID] = agent
	return nil
}

// FindByCapability returns all agents that have the named capability.
func (t *Tracker) FindByCapability(capability string) []AgentInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agentIDs, ok := t.byCapability[capability]
	if !ok {
		return nil
	}

	result := make([]AgentInfo, 0, len(agentIDs))
	for id := range agentIDs {
		result = append(result, t.agents[id])
	}
	return result
}

// FindInSpace returns all agents registered in the given space.
func (t *Tracker) FindInSpace(spaceID string) []AgentInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agentIDs, ok := t.bySpace[spaceID]
	if !ok {
		return nil
	}

	result := make([]AgentInfo, 0, len(agentIDs))
	for id := range agentIDs {
		result = append(result, t.agents[id])
	}
	return result
}

// Get returns a single agent by ID.
func (t *Tracker) Get(agentID string) (AgentInfo, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agent, ok := t.agents[agentID]
	return agent, ok
}

// All returns every registered agent.
func (t *Tracker) All() []AgentInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]AgentInfo, 0, len(t.agents))
	for _, agent := range t.agents {
		result = append(result, agent)
	}
	return result
}

// Count returns the number of registered agents.
func (t *Tracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.agents)
}
