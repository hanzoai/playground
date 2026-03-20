-- +goose Up
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    space_id    TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    state       TEXT NOT NULL DEFAULT 'pending',
    priority    INTEGER DEFAULT 0,
    assigned_to TEXT DEFAULT '',
    created_by  TEXT DEFAULT '',
    workflow_id TEXT DEFAULT '',
    parent_task_id TEXT DEFAULT '',
    depends_on  JSONB DEFAULT '[]',
    labels      JSONB DEFAULT '[]',
    input       JSONB DEFAULT '{}',
    output      JSONB DEFAULT '{}',
    error       TEXT DEFAULT '',
    progress    INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    timeout     BIGINT DEFAULT 0,
    metadata    JSONB DEFAULT '{}',
    started_at  TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_space_id ON tasks(space_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state ON tasks(state);
CREATE INDEX IF NOT EXISTS idx_tasks_space_state ON tasks(space_id, state);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_id ON tasks(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);

CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    space_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT DEFAULT '',
    state       TEXT NOT NULL DEFAULT 'pending',
    tasks       JSONB DEFAULT '[]',
    created_by  TEXT DEFAULT '',
    metadata    JSONB DEFAULT '{}',
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflows_space_id ON workflows(space_id);

-- +goose Down
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS tasks;
