-- +goose Up
-- Spaces: IAM-scoped project workspaces

CREATE TABLE IF NOT EXISTS spaces (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_spaces_org ON spaces(org_id);
CREATE UNIQUE INDEX idx_spaces_org_slug ON spaces(org_id, slug);

CREATE TABLE IF NOT EXISTS space_members (
    space_id   TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member','viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (space_id, user_id)
);

CREATE INDEX idx_space_members_user ON space_members(user_id);

CREATE TABLE IF NOT EXISTS space_nodes (
    space_id      TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    node_id       TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    type          TEXT NOT NULL DEFAULT 'local' CHECK (type IN ('local','cloud')),
    endpoint      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline','provisioning')),
    os            TEXT NOT NULL DEFAULT '',
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (space_id, node_id)
);

CREATE TABLE IF NOT EXISTS space_bots (
    space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    bot_id   TEXT NOT NULL,
    node_id  TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    name     TEXT NOT NULL DEFAULT '',
    model    TEXT NOT NULL DEFAULT '',
    view     TEXT NOT NULL DEFAULT 'terminal',
    status   TEXT NOT NULL DEFAULT 'stopped',
    PRIMARY KEY (space_id, bot_id)
);

-- +goose Down
DROP TABLE IF EXISTS space_bots;
DROP TABLE IF EXISTS space_nodes;
DROP TABLE IF EXISTS space_members;
DROP TABLE IF EXISTS spaces;
