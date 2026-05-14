-- 000011_workspaces_finalise.down.sql
-- Local-dev convenience only (per CLAUDE.md: down migrations are not for
-- production). Restores the pre-Step-3 shape: publisher_id columns + the
-- old (publisher_id, slug) unique key + the idx_*_publisher indexes.
--
-- The publisher_id value for each row is reconstructed via JOIN through
-- workspaces.publisher_id. This is loss-free as long as no code path ever
-- mutated workspaces.publisher_id after backfill (none exists today).

BEGIN;

-- ── Recreate publisher_id columns (nullable while we backfill) ────────────
ALTER TABLE mcp_servers
    ADD COLUMN publisher_id TEXT REFERENCES publishers(id) ON DELETE RESTRICT;
ALTER TABLE agents
    ADD COLUMN publisher_id TEXT REFERENCES publishers(id) ON DELETE RESTRICT;

-- ── Backfill from workspaces ──────────────────────────────────────────────
UPDATE mcp_servers s
   SET publisher_id = w.publisher_id
  FROM workspaces w
 WHERE w.id = s.workspace_id;

UPDATE agents a
   SET publisher_id = w.publisher_id
  FROM workspaces w
 WHERE w.id = a.workspace_id;

ALTER TABLE mcp_servers ALTER COLUMN publisher_id SET NOT NULL;
ALTER TABLE agents      ALTER COLUMN publisher_id SET NOT NULL;

-- ── Restore old unique key + indexes ──────────────────────────────────────
ALTER TABLE mcp_servers
    DROP CONSTRAINT mcp_servers_workspace_id_slug_key;
ALTER TABLE agents
    DROP CONSTRAINT agents_workspace_id_slug_key;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_publisher_id_slug_key UNIQUE (publisher_id, slug);
ALTER TABLE agents
    ADD CONSTRAINT agents_publisher_id_slug_key      UNIQUE (publisher_id, slug);

CREATE INDEX idx_mcp_servers_publisher ON mcp_servers (publisher_id);
CREATE INDEX idx_agents_publisher      ON agents (publisher_id);

COMMIT;
