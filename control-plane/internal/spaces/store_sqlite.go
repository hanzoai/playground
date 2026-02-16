package spaces

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLiteStore implements Store using SQLite for local development.
// In local mode, an implicit default space is created automatically.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a Store backed by an existing SQLite *sql.DB.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// Initialize creates the spaces schema and ensures a default local space exists.
func (s *SQLiteStore) Initialize(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS spaces (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL DEFAULT 'local',
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT 'local',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_spaces_org ON spaces(org_id)`,
		`CREATE TABLE IF NOT EXISTS space_members (
			space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (space_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS space_nodes (
			space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
			node_id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'local',
			endpoint TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'offline',
			os TEXT NOT NULL DEFAULT '',
			registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (space_id, node_id)
		)`,
		`CREATE TABLE IF NOT EXISTS space_bots (
			space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
			bot_id TEXT NOT NULL,
			node_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			view TEXT NOT NULL DEFAULT 'terminal',
			status TEXT NOT NULL DEFAULT 'stopped',
			PRIMARY KEY (space_id, bot_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite spaces schema: %w", err)
		}
	}

	// Ensure implicit default space for local mode
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO spaces (id, org_id, name, slug, description, created_by)
		 VALUES ('default', 'local', 'Default Space', 'default', 'Implicit local development space', 'local')`)
	return err
}

// --- Space CRUD ---

func (s *SQLiteStore) CreateSpace(ctx context.Context, space *Space) error {
	now := time.Now().UTC()
	space.CreatedAt = now
	space.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO spaces (id, org_id, name, slug, description, created_by, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		space.ID, space.OrgID, space.Name, space.Slug, space.Description,
		space.CreatedBy, space.CreatedAt, space.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetSpace(ctx context.Context, id string) (*Space, error) {
	var sp Space
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		 FROM spaces WHERE id = ?`, id,
	).Scan(&sp.ID, &sp.OrgID, &sp.Name, &sp.Slug, &sp.Description,
		&sp.CreatedBy, &sp.CreatedAt, &sp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("space %q not found", id)
	}
	return &sp, err
}

func (s *SQLiteStore) ListSpaces(ctx context.Context, orgID string) ([]*Space, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		 FROM spaces WHERE org_id = ? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Space
	for rows.Next() {
		var sp Space
		if err := rows.Scan(&sp.ID, &sp.OrgID, &sp.Name, &sp.Slug, &sp.Description,
			&sp.CreatedBy, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, &sp)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateSpace(ctx context.Context, space *Space) error {
	space.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE spaces SET name=?, slug=?, description=?, updated_at=? WHERE id=?`,
		space.Name, space.Slug, space.Description, space.UpdatedAt, space.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("space %q not found", space.ID)
	}
	return nil
}

func (s *SQLiteStore) DeleteSpace(ctx context.Context, id string) error {
	if id == "default" {
		return fmt.Errorf("cannot delete the default local space")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM spaces WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("space %q not found", id)
	}
	return nil
}

// --- Members ---

func (s *SQLiteStore) AddMember(ctx context.Context, m *SpaceMember) error {
	m.CreatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO space_members (space_id, user_id, role, created_at) VALUES (?,?,?,?)`,
		m.SpaceID, m.UserID, m.Role, m.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) RemoveMember(ctx context.Context, spaceID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_members WHERE space_id=? AND user_id=?`, spaceID, userID)
	return err
}

func (s *SQLiteStore) ListMembers(ctx context.Context, spaceID string) ([]*SpaceMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, user_id, role, created_at FROM space_members WHERE space_id=?`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SpaceMember
	for rows.Next() {
		var m SpaceMember
		if err := rows.Scan(&m.SpaceID, &m.UserID, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &m)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetMember(ctx context.Context, spaceID, userID string) (*SpaceMember, error) {
	var m SpaceMember
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, user_id, role, created_at FROM space_members WHERE space_id=? AND user_id=?`,
		spaceID, userID,
	).Scan(&m.SpaceID, &m.UserID, &m.Role, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

// --- Nodes ---

func (s *SQLiteStore) RegisterNode(ctx context.Context, node *SpaceNode) error {
	node.RegisteredAt = time.Now().UTC()
	node.LastSeen = node.RegisteredAt
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO space_nodes (space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		node.SpaceID, node.NodeID, node.Name, node.Type, node.Endpoint,
		node.Status, node.OS, node.RegisteredAt, node.LastSeen,
	)
	return err
}

func (s *SQLiteStore) GetNode(ctx context.Context, spaceID, nodeID string) (*SpaceNode, error) {
	var n SpaceNode
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen
		 FROM space_nodes WHERE space_id=? AND node_id=?`, spaceID, nodeID,
	).Scan(&n.SpaceID, &n.NodeID, &n.Name, &n.Type, &n.Endpoint,
		&n.Status, &n.OS, &n.RegisteredAt, &n.LastSeen)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %q not found in space %q", nodeID, spaceID)
	}
	return &n, err
}

func (s *SQLiteStore) ListNodes(ctx context.Context, spaceID string) ([]*SpaceNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen
		 FROM space_nodes WHERE space_id=? ORDER BY registered_at DESC`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SpaceNode
	for rows.Next() {
		var n SpaceNode
		if err := rows.Scan(&n.SpaceID, &n.NodeID, &n.Name, &n.Type, &n.Endpoint,
			&n.Status, &n.OS, &n.RegisteredAt, &n.LastSeen); err != nil {
			return nil, err
		}
		result = append(result, &n)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateNodeStatus(ctx context.Context, spaceID, nodeID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE space_nodes SET status=?, last_seen=? WHERE space_id=? AND node_id=?`,
		status, time.Now().UTC(), spaceID, nodeID)
	return err
}

func (s *SQLiteStore) RemoveNode(ctx context.Context, spaceID, nodeID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_nodes WHERE space_id=? AND node_id=?`, spaceID, nodeID)
	return err
}

// --- Bots ---

func (s *SQLiteStore) CreateBot(ctx context.Context, bot *SpaceBot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO space_bots (space_id, bot_id, node_id, agent_id, name, model, view, status)
		 VALUES (?,?,?,?,?,?,?,?)`,
		bot.SpaceID, bot.BotID, bot.NodeID, bot.AgentID,
		bot.Name, bot.Model, bot.View, bot.Status,
	)
	return err
}

func (s *SQLiteStore) GetBot(ctx context.Context, spaceID, botID string) (*SpaceBot, error) {
	var b SpaceBot
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, bot_id, node_id, agent_id, name, model, view, status
		 FROM space_bots WHERE space_id=? AND bot_id=?`, spaceID, botID,
	).Scan(&b.SpaceID, &b.BotID, &b.NodeID, &b.AgentID, &b.Name, &b.Model, &b.View, &b.Status)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bot %q not found in space %q", botID, spaceID)
	}
	return &b, err
}

func (s *SQLiteStore) ListBots(ctx context.Context, spaceID string) ([]*SpaceBot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, bot_id, node_id, agent_id, name, model, view, status
		 FROM space_bots WHERE space_id=?`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SpaceBot
	for rows.Next() {
		var b SpaceBot
		if err := rows.Scan(&b.SpaceID, &b.BotID, &b.NodeID, &b.AgentID,
			&b.Name, &b.Model, &b.View, &b.Status); err != nil {
			return nil, err
		}
		result = append(result, &b)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateBotStatus(ctx context.Context, spaceID, botID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE space_bots SET status=? WHERE space_id=? AND bot_id=?`,
		status, spaceID, botID)
	return err
}

func (s *SQLiteStore) RemoveBot(ctx context.Context, spaceID, botID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_bots WHERE space_id=? AND bot_id=?`, spaceID, botID)
	return err
}
