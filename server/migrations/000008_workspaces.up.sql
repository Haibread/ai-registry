-- 000008_workspaces.up.sql
-- Phase 7.1 step 1: introduce the workspace entity between publishers
-- and resources.
--
-- This migration is the SCHEMA half of a two-step rollout:
--   1. (here) create the workspaces table; add nullable workspace_id to
--      mcp_servers and agents.
--   2. (later) Go-side backfill creates a 'default' workspace per publisher
--      and rewrites the FKs.
--   3. (later) finalising migration flips workspace_id to NOT NULL and drops
--      publisher_id from resources.
--
-- Until the finalising migration lands, both `publisher_id` and the new
-- `workspace_id` columns coexist on mcp_servers and agents. New code MUST
-- continue writing publisher_id; the workspace_id column stays NULL until
-- the backfill runs.

-- ── Workspaces ────────────────────────────────────────────────────────────

CREATE TABLE workspaces (
    id           TEXT        PRIMARY KEY,
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    contact      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── Add nullable workspace_id to existing resource tables ─────────────────

ALTER TABLE mcp_servers
    ADD COLUMN workspace_id TEXT
    REFERENCES workspaces(id) ON DELETE RESTRICT;

CREATE INDEX idx_mcp_servers_workspace ON mcp_servers (workspace_id);

ALTER TABLE agents
    ADD COLUMN workspace_id TEXT
    REFERENCES workspaces(id) ON DELETE RESTRICT;

CREATE INDEX idx_agents_workspace ON agents (workspace_id);
