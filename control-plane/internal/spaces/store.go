package spaces

import "context"

// Store is the persistence interface for Space entities.
// Implemented by both SQLite (local) and PostgreSQL (cloud) backends.
type Store interface {
	// Space CRUD
	CreateSpace(ctx context.Context, space *Space) error
	GetSpace(ctx context.Context, id string) (*Space, error)
	ListSpaces(ctx context.Context, orgID string) ([]*Space, error)
	UpdateSpace(ctx context.Context, space *Space) error
	DeleteSpace(ctx context.Context, id string) error

	// Members
	AddMember(ctx context.Context, member *SpaceMember) error
	RemoveMember(ctx context.Context, spaceID, userID string) error
	ListMembers(ctx context.Context, spaceID string) ([]*SpaceMember, error)
	GetMember(ctx context.Context, spaceID, userID string) (*SpaceMember, error)

	// Nodes
	RegisterNode(ctx context.Context, node *SpaceNode) error
	GetNode(ctx context.Context, spaceID, nodeID string) (*SpaceNode, error)
	ListNodes(ctx context.Context, spaceID string) ([]*SpaceNode, error)
	UpdateNodeStatus(ctx context.Context, spaceID, nodeID, status string) error
	RemoveNode(ctx context.Context, spaceID, nodeID string) error

	// Bots
	CreateBot(ctx context.Context, bot *SpaceBot) error
	GetBot(ctx context.Context, spaceID, botID string) (*SpaceBot, error)
	ListBots(ctx context.Context, spaceID string) ([]*SpaceBot, error)
	UpdateBotStatus(ctx context.Context, spaceID, botID, status string) error
	RemoveBot(ctx context.Context, spaceID, botID string) error

	// Initialize schema
	Initialize(ctx context.Context) error
}
