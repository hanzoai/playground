// Package spaces provides IAM-scoped project workspaces ("Spaces")
// that organize nodes, bots, and team members under an organization.
package spaces

import "time"

// Space is an IAM-scoped project workspace within an org.
type Space struct {
	ID          string    `json:"id" db:"id"`
	OrgID       string    `json:"org_id" db:"org_id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description" db:"description"`
	CreatedBy   string    `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// SpaceMember represents a user's access to a Space.
type SpaceMember struct {
	SpaceID   string    `json:"space_id" db:"space_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Role      string    `json:"role" db:"role"` // "owner", "admin", "member", "viewer"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// SpaceNode is a hanzo/node runtime registered to a Space.
type SpaceNode struct {
	SpaceID      string    `json:"space_id" db:"space_id"`
	NodeID       string    `json:"node_id" db:"node_id"`
	Name         string    `json:"name" db:"name"`
	Type         string    `json:"type" db:"type"`         // "local", "cloud"
	Endpoint     string    `json:"endpoint" db:"endpoint"` // hanzo/node HTTP API URL
	Status       string    `json:"status" db:"status"`     // "online", "offline", "provisioning"
	OS           string    `json:"os" db:"os"`             // "linux", "macos", "windows"
	RegisteredAt time.Time `json:"registered_at" db:"registered_at"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
}

// SpaceBot is an agent process assigned to a Space, running on a node.
type SpaceBot struct {
	SpaceID string `json:"space_id" db:"space_id"`
	BotID   string `json:"bot_id" db:"bot_id"`
	NodeID  string `json:"node_id" db:"node_id"`   // which node it runs on
	AgentID string `json:"agent_id" db:"agent_id"` // hanzo/node agent_id
	Name    string `json:"name" db:"name"`
	Model   string `json:"model" db:"model"`
	View    string `json:"view" db:"view"`     // "terminal", "desktop-linux", "desktop-mac", "desktop-win", "chat"
	Status  string `json:"status" db:"status"` // "running", "stopped", "error"
}

// ValidRoles for SpaceMember.
var ValidRoles = map[string]bool{
	"owner":  true,
	"admin":  true,
	"member": true,
	"viewer": true,
}

// ValidNodeTypes for SpaceNode.
var ValidNodeTypes = map[string]bool{
	"local": true,
	"cloud": true,
}

// ValidViews for SpaceBot.
var ValidViews = map[string]bool{
	"terminal":      true,
	"desktop-linux": true,
	"desktop-mac":   true,
	"desktop-win":   true,
	"chat":          true,
}
