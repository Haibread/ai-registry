-- 000011_workspaces_finalise.up.sql
-- ADR 0001 Step 3: finalise the workspaces migration started in 000008.
--
-- Pre-condition: every row in mcp_servers and agents must have a non-NULL
-- workspace_id. The Go-side BackfillWorkspaces (server/internal/store/
-- workspace.go) runs on boot and populates the column; this migration must
-- run AFTER that has executed at least once against the target database.
--
-- This migration:
--   1. Gates on the backfill having completed (RAISE EXCEPTION otherwise).
--   2. Flips workspace_id to NOT NULL on mcp_servers and agents.
--   3. Replaces the old UNIQUE(publisher_id, slug) with UNIQUE(workspace_id,
--      slug) — per ADR 0001, slug uniqueness is per-workspace, so two
--      workspaces under one publisher can both expose a `weather` server.
--   4. Drops the now-unused publisher_id column (and its index/FK, cascaded).
--
-- After this lands, mcp_servers and agents reach their owning publisher via
-- workspaces.publisher_id (a single JOIN). The workspaces table itself keeps
-- its publisher_id column — that link survives.

BEGIN;

-- ── Sanity gate ───────────────────────────────────────────────────────────
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM mcp_servers WHERE workspace_id IS NULL)
       OR EXISTS (SELECT 1 FROM agents WHERE workspace_id IS NULL) THEN
        RAISE EXCEPTION
            'workspace backfill not complete — boot the server once to run BackfillWorkspaces against this database, then retry the migration';
    END IF;
END $$;

-- ── Flip workspace_id to NOT NULL ─────────────────────────────────────────
ALTER TABLE mcp_servers ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE agents      ALTER COLUMN workspace_id SET NOT NULL;

-- ── Swap unique key from (publisher_id, slug) to (workspace_id, slug) ─────
ALTER TABLE mcp_servers DROP CONSTRAINT mcp_servers_publisher_id_slug_key;
ALTER TABLE agents      DROP CONSTRAINT agents_publisher_id_slug_key;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_workspace_id_slug_key UNIQUE (workspace_id, slug);
ALTER TABLE agents
    ADD CONSTRAINT agents_workspace_id_slug_key      UNIQUE (workspace_id, slug);

-- ── Drop publisher_id (cascades the publisher FK + idx_*_publisher) ───────
DROP INDEX IF EXISTS idx_mcp_servers_publisher;
DROP INDEX IF EXISTS idx_agents_publisher;

ALTER TABLE mcp_servers DROP COLUMN publisher_id;
ALTER TABLE agents      DROP COLUMN publisher_id;

COMMIT;
