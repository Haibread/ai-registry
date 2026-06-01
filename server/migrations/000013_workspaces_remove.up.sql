-- 000013_workspaces_remove.up.sql
-- Finalise: remove the workspace layer entirely. Resources
-- become publisher-scoped again, reversing the earlier finalising migration
-- (000011). Forward-only; the down migration exists for local-dev convenience.
--
-- Pre-condition: publisher_id was re-added + backfilled in 000012. This
-- migration re-backfills first (catching rows created after 000012 by code
-- that still only wrote workspace_id) so the NOT NULL flip cannot fail on a
-- live upgrade, then gates, flips, swaps the unique key, and drops the
-- workspace column + table.

BEGIN;

-- ── Re-backfill any rows missed since 000012 ──────────────────────────────
UPDATE mcp_servers s
    SET publisher_id = w.publisher_id
    FROM workspaces w
    WHERE w.id = s.workspace_id AND s.publisher_id IS NULL;
UPDATE agents a
    SET publisher_id = w.publisher_id
    FROM workspaces w
    WHERE w.id = a.workspace_id AND a.publisher_id IS NULL;

-- ── Gate 1: every resource must have a publisher_id ───────────────────────
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM mcp_servers WHERE publisher_id IS NULL)
       OR EXISTS (SELECT 1 FROM agents WHERE publisher_id IS NULL) THEN
        RAISE EXCEPTION
            'publisher_id backfill incomplete — a resource has no owning publisher; boot the 000012 build once against this database, then retry';
    END IF;
END $$;

-- ── Gate 2: (publisher_id, slug) must be unique before we re-key on it ─────
-- A publisher that had two workspaces each exposing a same-slug resource would
-- collide. Fail loudly with the offending rows rather than lose one.
DO $$
DECLARE dup record;
BEGIN
    FOR dup IN
        SELECT publisher_id, slug, COUNT(*) AS c
        FROM mcp_servers GROUP BY publisher_id, slug HAVING COUNT(*) > 1
    LOOP
        RAISE EXCEPTION 'duplicate (publisher_id, slug) in mcp_servers: %/% (% rows) — resolve before finalising',
            dup.publisher_id, dup.slug, dup.c;
    END LOOP;
    FOR dup IN
        SELECT publisher_id, slug, COUNT(*) AS c
        FROM agents GROUP BY publisher_id, slug HAVING COUNT(*) > 1
    LOOP
        RAISE EXCEPTION 'duplicate (publisher_id, slug) in agents: %/% (% rows) — resolve before finalising',
            dup.publisher_id, dup.slug, dup.c;
    END LOOP;
END $$;

-- ── Flip publisher_id to NOT NULL ─────────────────────────────────────────
ALTER TABLE mcp_servers ALTER COLUMN publisher_id SET NOT NULL;
ALTER TABLE agents      ALTER COLUMN publisher_id SET NOT NULL;

-- ── Swap unique key from (workspace_id, slug) back to (publisher_id, slug) ─
ALTER TABLE mcp_servers DROP CONSTRAINT mcp_servers_workspace_id_slug_key;
ALTER TABLE agents      DROP CONSTRAINT agents_workspace_id_slug_key;
ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_publisher_id_slug_key UNIQUE (publisher_id, slug);
ALTER TABLE agents
    ADD CONSTRAINT agents_publisher_id_slug_key      UNIQUE (publisher_id, slug);

-- ── Drop workspace_id (cascades its FK; drop its index first) ─────────────
DROP INDEX IF EXISTS idx_mcp_servers_workspace;
DROP INDEX IF EXISTS idx_agents_workspace;
ALTER TABLE mcp_servers DROP COLUMN workspace_id;
ALTER TABLE agents      DROP COLUMN workspace_id;

-- ── Drop the workspaces table ──────────────────────────────────────────────
DROP TABLE workspaces;

COMMIT;
