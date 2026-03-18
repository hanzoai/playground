-- +goose Up
-- Chat messages for Space chat rooms

CREATE TABLE IF NOT EXISTS chat_messages (
    id           TEXT PRIMARY KEY,
    space_id     TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    message      TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_space_time ON chat_messages(space_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS chat_messages;
