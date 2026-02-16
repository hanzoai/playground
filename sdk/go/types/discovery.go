package types

import "time"

// DiscoveryPagination mirrors the pagination metadata returned by the control plane.
type DiscoveryPagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// DiscoveryResponse is the full JSON discovery payload.
type DiscoveryResponse struct {
	DiscoveredAt   time.Time           `json:"discovered_at"`
	TotalAgents    int                 `json:"total_agents"`
	TotalBots int                 `json:"total_bots"`
	TotalSkills    int                 `json:"total_skills"`
	Pagination     DiscoveryPagination `json:"pagination"`
	Capabilities   []NodeCapability  `json:"capabilities"`
}

// NodeCapability represents an individual node and its capabilities.
// The AgentID JSON field is retained for backward wire compatibility.
type NodeCapability struct {
	AgentID        string               `json:"agent_id"`
	BaseURL        string               `json:"base_url"`
	Version        string               `json:"version"`
	HealthStatus   string               `json:"health_status"`
	DeploymentType string               `json:"deployment_type"`
	LastHeartbeat  time.Time            `json:"last_heartbeat"`
	Bots           []BotCapability      `json:"bots"`
	Skills         []SkillCapability    `json:"skills"`
}

// BotCapability contains metadata for a bot.
type BotCapability struct {
	ID               string                   `json:"id"`
	Description      *string                  `json:"description,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	InputSchema      map[string]interface{}   `json:"input_schema,omitempty"`
	OutputSchema     map[string]interface{}   `json:"output_schema,omitempty"`
	Examples         []map[string]interface{} `json:"examples,omitempty"`
	InvocationTarget string                   `json:"invocation_target"`
}

// SkillCapability contains metadata for a skill.
type SkillCapability struct {
	ID               string                 `json:"id"`
	Description      *string                `json:"description,omitempty"`
	Tags             []string               `json:"tags,omitempty"`
	InputSchema      map[string]interface{} `json:"input_schema,omitempty"`
	InvocationTarget string                 `json:"invocation_target"`
}

// CompactDiscoveryResponse is returned when requesting the compact format.
type CompactDiscoveryResponse struct {
	DiscoveredAt time.Time           `json:"discovered_at"`
	Bots    []CompactCapability `json:"bots"`
	Skills       []CompactCapability `json:"skills"`
}

// CompactCapability is the minimal capability representation.
type CompactCapability struct {
	ID      string   `json:"id"`
	AgentID string   `json:"agent_id"`
	Target  string   `json:"target"`
	Tags    []string `json:"tags,omitempty"`
}

// DiscoveryResult wraps the possible discovery formats.
type DiscoveryResult struct {
	Format  string
	JSON    *DiscoveryResponse
	Compact *CompactDiscoveryResponse
	XML     string
	Raw     string
}
