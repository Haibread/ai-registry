-- 000013_workspaces_remove.down.sql
-- Reverses 000013. Local-dev convenience only — never relied on in production.
-- Recreates the workspaces table with one 'default' workspace per publisher and
-- re-points resources at it, restoring the post-000012 shape (both publisher_id
-- and workspace_id present). Data created since the up-migration that relied on
-- per-workspace slug uniqueness cannot be perfectly reconstructed.

BEGIN;

-- ── Recreate workspaces (mirrors 000008 + the 000009 group_name column) ────
CREATE TABLE workspaces (
    id           TEXT        PRIMARY KEY,
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    contact      TEXT,
    group_name   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX workspaces_group_name_idx
    ON workspaces (group_name) WHERE group_name IS NOT NULL;

-- One 'default' workspace per publisher.
INSERT INTO workspaces (id, publisher_id, slug, name)
SELECT gen_random_uuid()::text, p.id, 'default', 'Default workspace'
FROM publishers p;

-- ── Re-add workspace_id, backfill from the default workspace ──────────────
ALTER TABLE mcp_servers
    ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE RESTRICT;
ALTER TABLE agents
    ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE RESTRICT;
CREATE INDEX idx_mcp_servers_workspace ON mcp_servers (workspace_id);
CREATE INDEX idx_agents_workspace      ON agents (workspace_id);

UPDATE mcp_servers s
    SET workspace_id = w.id
    FROM workspaces w
    WHERE w.publisher_id = s.publisher_id AND w.slug = 'default';
UPDATE agents a
    SET workspace_id = w.id
    FROM workspaces w
    WHERE w.publisher_id = a.publisher_id AND w.slug = 'default';

ALTER TABLE mcp_servers ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE agents      ALTER COLUMN workspace_id SET NOT NULL;

-- ── Swap unique key back to (workspace_id, slug); publisher_id nullable ────
ALTER TABLE mcp_servers DROP CONSTRAINT mcp_servers_publisher_id_slug_key;
ALTER TABLE agents      DROP CONSTRAINT agents_publisher_id_slug_key;
ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_workspace_id_slug_key UNIQUE (workspace_id, slug);
ALTER TABLE agents
    ADD CONSTRAINT agents_workspace_id_slug_key      UNIQUE (workspace_id, slug);
ALTER TABLE mcp_servers ALTER COLUMN publisher_id DROP NOT NULL;
ALTER TABLE agents      ALTER COLUMN publisher_id DROP NOT NULL;

COMMIT;
