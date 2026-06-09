-- 000017_slug_reuse_after_delete.down.sql
-- Restore the full UNIQUE (publisher_id, slug) constraint.
--
-- Local-dev convenience only. This will fail if a publisher currently has both
-- a deleted and a live (or multiple deleted) row sharing the same slug — drop
-- the surplus rows before reverting.

BEGIN;

DROP INDEX IF EXISTS mcp_servers_publisher_slug_live;
DROP INDEX IF EXISTS agents_publisher_slug_live;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_publisher_id_slug_key UNIQUE (publisher_id, slug);
ALTER TABLE agents
    ADD CONSTRAINT agents_publisher_id_slug_key      UNIQUE (publisher_id, slug);

COMMIT;
