// Package policy provides the permissions engine for the Hanzo Playground
// control plane. It determines what bots can access, how DNS is managed,
// and whether human approval is required for actions.
package policy

import "time"

// ResourceType is a class of infrastructure resource bots can access.
type ResourceType string

const (
	ResourceDNS     ResourceType = "dns"
	ResourceKMS     ResourceType = "kms"
	ResourceCloud   ResourceType = "cloud"   // cloud-api, compute
	ResourceIAM     ResourceType = "iam"     // identity management
	ResourceGit     ResourceType = "git"     // repo operations
	ResourceNetwork ResourceType = "network" // outbound HTTP/WS
	ResourceShell   ResourceType = "shell"   // command execution
	ResourceFiles   ResourceType = "files"   // filesystem
	ResourceSecrets ResourceType = "secrets" // KMS secrets
	ResourceDeploy  ResourceType = "deploy"  // k8s, paas deploys
	ResourceBilling ResourceType = "billing" // spend money
)

// Permission is a verb on a resource.
type Permission string

const (
	PermRead    Permission = "read"
	PermWrite   Permission = "write"
	PermExecute Permission = "execute"
	PermAdmin   Permission = "admin"
	PermDeny    Permission = "deny"
)

// permLevel returns a numeric level for permission comparison.
// Higher level implies all lower permissions are granted.
// PermDeny is special and handled separately.
func permLevel(p Permission) int {
	switch p {
	case PermRead:
		return 1
	case PermWrite:
		return 2
	case PermExecute:
		return 3
	case PermAdmin:
		return 4
	default:
		return 0
	}
}

// ApprovalMode controls how bot actions are gated.
type ApprovalMode string

const (
	ApprovalManaged ApprovalMode = "managed" // humans approve writes
	ApprovalTrusted ApprovalMode = "trusted" // auto-approve known-safe
	ApprovalBypass  ApprovalMode = "bypass"  // bots go wild
)

// PolicyRule is a single permission rule for a bot on a resource.
type PolicyRule struct {
	Resource   ResourceType `json:"resource"`
	Permission Permission   `json:"permission"`
	Conditions []Condition  `json:"conditions,omitempty"` // optional constraints
}

// Condition is an optional constraint on a rule (e.g., max spend, allowed domains).
type Condition struct {
	Type  string `json:"type"`  // "max_spend", "allowed_domains", "allowed_paths", "time_window"
	Value string `json:"value"` // JSON-encoded value
}

// BotPolicy is the full policy for a single bot.
type BotPolicy struct {
	BotID          string       `json:"bot_id"`
	SpaceID        string       `json:"space_id"`
	ApprovalMode   ApprovalMode `json:"approval_mode"`
	Rules          []PolicyRule `json:"rules"`
	MaxSpendUSD    float64      `json:"max_spend_usd,omitempty"`    // 0 = unlimited
	AllowedDomains []string     `json:"allowed_domains,omitempty"`  // DNS/network
	DenyPatterns   []string     `json:"deny_patterns,omitempty"`    // glob patterns to deny
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// SpacePolicy is the default policy for all bots in a space.
type SpacePolicy struct {
	SpaceID      string       `json:"space_id"`
	ApprovalMode ApprovalMode `json:"approval_mode"` // default for new bots
	DefaultRules []PolicyRule `json:"default_rules"`
	MaxSpendUSD  float64      `json:"max_spend_usd,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ApprovalRequest is a bot asking a human to approve an action.
type ApprovalRequest struct {
	ID          string       `json:"id"`
	BotID       string       `json:"bot_id"`
	SpaceID     string       `json:"space_id"`
	Resource    ResourceType `json:"resource"`
	Permission  Permission   `json:"permission"`
	Description string       `json:"description"`           // human-readable
	Command     string       `json:"command,omitempty"`      // for shell commands
	Status      string       `json:"status"`                 // "pending", "approved", "denied", "expired"
	RequestedAt time.Time    `json:"requested_at"`
	ResolvedAt  *time.Time   `json:"resolved_at,omitempty"`
	ResolvedBy  string       `json:"resolved_by,omitempty"`  // human who approved/denied
}

// Approval status constants.
const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusDenied   = "denied"
	ApprovalStatusExpired  = "expired"
)
