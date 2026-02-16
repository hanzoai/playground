-- Migration: Rename agent terminology to hanzo/playground
-- Description: Renames tables and columns from agent_* to hanzo/playground terminology
-- Created: 2026-02-15

-- +goose Up

-- Rename agent_dids table to hanzo_dids
ALTER TABLE agent_dids RENAME TO hanzo_dids;

-- Rename agent_node_id columns across tables
ALTER TABLE hanzo_dids RENAME COLUMN agent_node_id TO node_id;
ALTER TABLE executions RENAME COLUMN agent_node_id TO node_id;
ALTER TABLE agent_executions RENAME COLUMN agent_node_id TO node_id;
ALTER TABLE workflow_executions RENAME COLUMN agent_node_id TO node_id;
ALTER TABLE workflow_steps RENAME COLUMN agent_node_id TO node_id;

-- Rename agents_server_id columns
ALTER TABLE did_registry RENAME COLUMN agents_server_id TO playground_server_id;
ALTER TABLE hanzo_dids RENAME COLUMN agents_server_id TO playground_server_id;

-- Rename agent_nodes column in did_registry (JSON field tracking node DIDs)
ALTER TABLE did_registry RENAME COLUMN agent_nodes TO nodes;

-- Recreate indexes with new names (PostgreSQL does not rename indexes with columns)
DROP INDEX IF EXISTS idx_agent_dids_agent_node_id;
DROP INDEX IF EXISTS idx_agent_dids_organization_id;
DROP INDEX IF EXISTS idx_agent_dids_status;
DROP INDEX IF EXISTS idx_agent_dids_registered_at;
DROP INDEX IF EXISTS idx_agent_dids_agent_node_org;
DROP INDEX IF EXISTS idx_agent_dids_agent_node;
DROP INDEX IF EXISTS idx_agent_dids_agents_server;

CREATE INDEX IF NOT EXISTS idx_hanzo_dids_node_id ON hanzo_dids(node_id);
CREATE INDEX IF NOT EXISTS idx_hanzo_dids_playground_server_id ON hanzo_dids(playground_server_id);
CREATE INDEX IF NOT EXISTS idx_hanzo_dids_status ON hanzo_dids(status);
CREATE INDEX IF NOT EXISTS idx_hanzo_dids_registered_at ON hanzo_dids(registered_at);

-- Update workflow_executions indexes
DROP INDEX IF EXISTS idx_workflow_executions_agent_node;
DROP INDEX IF EXISTS idx_workflow_executions_agent_node_status;

CREATE INDEX IF NOT EXISTS idx_workflow_executions_node ON workflow_executions(node_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_node_status ON workflow_executions(node_id, status);

-- +goose Down

-- Revert workflow_executions indexes
DROP INDEX IF EXISTS idx_workflow_executions_node;
DROP INDEX IF EXISTS idx_workflow_executions_node_status;

CREATE INDEX IF NOT EXISTS idx_workflow_executions_agent_node ON workflow_executions(agent_node_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_agent_node_status ON workflow_executions(agent_node_id, status);

-- Revert hanzo_dids indexes
DROP INDEX IF EXISTS idx_hanzo_dids_node_id;
DROP INDEX IF EXISTS idx_hanzo_dids_playground_server_id;
DROP INDEX IF EXISTS idx_hanzo_dids_status;
DROP INDEX IF EXISTS idx_hanzo_dids_registered_at;

CREATE INDEX IF NOT EXISTS idx_agent_dids_agent_node_id ON agent_dids(agent_node_id);
CREATE INDEX IF NOT EXISTS idx_agent_dids_organization_id ON agent_dids(agents_server_id);
CREATE INDEX IF NOT EXISTS idx_agent_dids_status ON agent_dids(status);
CREATE INDEX IF NOT EXISTS idx_agent_dids_registered_at ON agent_dids(registered_at);

-- Revert column renames
ALTER TABLE did_registry RENAME COLUMN nodes TO agent_nodes;
ALTER TABLE hanzo_dids RENAME COLUMN playground_server_id TO agents_server_id;
ALTER TABLE did_registry RENAME COLUMN playground_server_id TO agents_server_id;

ALTER TABLE workflow_steps RENAME COLUMN node_id TO agent_node_id;
ALTER TABLE workflow_executions RENAME COLUMN node_id TO agent_node_id;
ALTER TABLE agent_executions RENAME COLUMN node_id TO agent_node_id;
ALTER TABLE executions RENAME COLUMN node_id TO agent_node_id;
ALTER TABLE hanzo_dids RENAME COLUMN node_id TO agent_node_id;

-- Revert table rename
ALTER TABLE hanzo_dids RENAME TO agent_dids;
