package spaces

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a Store backed by an existing *sql.DB connection.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Initialize(ctx context.Context) error {
	// Schema created via goose migrations (019_create_spaces.sql).
	return nil
}

// --- Space CRUD ---

func (s *PostgresStore) CreateSpace(ctx context.Context, space *Space) error {
	now := time.Now().UTC()
	space.CreatedAt = now
	space.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO spaces (id, org_id, name, slug, description, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		space.ID, space.OrgID, space.Name, space.Slug, space.Description,
		space.CreatedBy, space.CreatedAt, space.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) GetSpace(ctx context.Context, id string) (*Space, error) {
	var sp Space
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		 FROM spaces WHERE id = $1`, id,
	).Scan(&sp.ID, &sp.OrgID, &sp.Name, &sp.Slug, &sp.Description,
		&sp.CreatedBy, &sp.CreatedAt, &sp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("space %q not found", id)
	}
	return &sp, err
}

func (s *PostgresStore) ListSpaces(ctx context.Context, orgID string) ([]*Space, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		 FROM spaces WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
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

func (s *PostgresStore) UpdateSpace(ctx context.Context, space *Space) error {
	space.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE spaces SET name=$1, slug=$2, description=$3, updated_at=$4 WHERE id=$5`,
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

func (s *PostgresStore) DeleteSpace(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM spaces WHERE id=$1`, id)
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

func (s *PostgresStore) AddMember(ctx context.Context, m *SpaceMember) error {
	m.CreatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO space_members (space_id, user_id, role, created_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (space_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		m.SpaceID, m.UserID, m.Role, m.CreatedAt,
	)
	return err
}

func (s *PostgresStore) RemoveMember(ctx context.Context, spaceID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_members WHERE space_id=$1 AND user_id=$2`, spaceID, userID)
	return err
}

func (s *PostgresStore) ListMembers(ctx context.Context, spaceID string) ([]*SpaceMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, user_id, role, created_at FROM space_members WHERE space_id=$1`, spaceID)
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

func (s *PostgresStore) GetMember(ctx context.Context, spaceID, userID string) (*SpaceMember, error) {
	var m SpaceMember
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, user_id, role, created_at FROM space_members WHERE space_id=$1 AND user_id=$2`,
		spaceID, userID,
	).Scan(&m.SpaceID, &m.UserID, &m.Role, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

// --- Nodes ---

func (s *PostgresStore) RegisterNode(ctx context.Context, node *SpaceNode) error {
	node.RegisteredAt = time.Now().UTC()
	node.LastSeen = node.RegisteredAt
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO space_nodes (space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (space_id, node_id) DO UPDATE SET
		   endpoint=EXCLUDED.endpoint, status=EXCLUDED.status, last_seen=EXCLUDED.last_seen`,
		node.SpaceID, node.NodeID, node.Name, node.Type, node.Endpoint,
		node.Status, node.OS, node.RegisteredAt, node.LastSeen,
	)
	return err
}

func (s *PostgresStore) GetNode(ctx context.Context, spaceID, nodeID string) (*SpaceNode, error) {
	var n SpaceNode
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen
		 FROM space_nodes WHERE space_id=$1 AND node_id=$2`, spaceID, nodeID,
	).Scan(&n.SpaceID, &n.NodeID, &n.Name, &n.Type, &n.Endpoint,
		&n.Status, &n.OS, &n.RegisteredAt, &n.LastSeen)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %q not found in space %q", nodeID, spaceID)
	}
	return &n, err
}

func (s *PostgresStore) ListNodes(ctx context.Context, spaceID string) ([]*SpaceNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, node_id, name, type, endpoint, status, os, registered_at, last_seen
		 FROM space_nodes WHERE space_id=$1 ORDER BY registered_at DESC`, spaceID)
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

func (s *PostgresStore) UpdateNodeStatus(ctx context.Context, spaceID, nodeID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE space_nodes SET status=$1, last_seen=$2 WHERE space_id=$3 AND node_id=$4`,
		status, time.Now().UTC(), spaceID, nodeID)
	return err
}

func (s *PostgresStore) RemoveNode(ctx context.Context, spaceID, nodeID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_nodes WHERE space_id=$1 AND node_id=$2`, spaceID, nodeID)
	return err
}

// --- Bots ---

func (s *PostgresStore) CreateBot(ctx context.Context, bot *SpaceBot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO space_bots (space_id, bot_id, node_id, agent_id, name, model, view, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		bot.SpaceID, bot.BotID, bot.NodeID, bot.AgentID,
		bot.Name, bot.Model, bot.View, bot.Status,
	)
	return err
}

func (s *PostgresStore) GetBot(ctx context.Context, spaceID, botID string) (*SpaceBot, error) {
	var b SpaceBot
	err := s.db.QueryRowContext(ctx,
		`SELECT space_id, bot_id, node_id, agent_id, name, model, view, status
		 FROM space_bots WHERE space_id=$1 AND bot_id=$2`, spaceID, botID,
	).Scan(&b.SpaceID, &b.BotID, &b.NodeID, &b.AgentID, &b.Name, &b.Model, &b.View, &b.Status)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bot %q not found in space %q", botID, spaceID)
	}
	return &b, err
}

func (s *PostgresStore) ListBots(ctx context.Context, spaceID string) ([]*SpaceBot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT space_id, bot_id, node_id, agent_id, name, model, view, status
		 FROM space_bots WHERE space_id=$1`, spaceID)
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

func (s *PostgresStore) UpdateBotStatus(ctx context.Context, spaceID, botID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE space_bots SET status=$1 WHERE space_id=$2 AND bot_id=$3`,
		status, spaceID, botID)
	return err
}

func (s *PostgresStore) RemoveBot(ctx context.Context, spaceID, botID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM space_bots WHERE space_id=$1 AND bot_id=$2`, spaceID, botID)
	return err
}
