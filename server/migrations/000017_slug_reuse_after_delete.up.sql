-- 000017_slug_reuse_after_delete.up.sql
-- Allow a slug to be reused after its MCP server / agent is soft-deleted.
--
-- Deletion is a soft delete (status='deleted'); the row is retained for audit.
-- The original table-level UNIQUE (publisher_id, slug) covered deleted rows
-- too, so a deleted entry kept its slug reserved forever and re-creating it
-- failed with a duplicate (23505) error. Replace the full constraint with a
-- partial unique index that only enforces uniqueness across live (non-deleted)
-- rows, so a publisher can re-create a previously deleted slug.
--
-- Single-row lookups by (publisher_id, slug) already prefer the live row: the
-- store getters and view/copy-count updates filter `status != 'deleted'`, and
-- ListMCPServers/ListAgents exclude deleted by default. Nothing references
-- (publisher_id, slug) as a foreign key (versions/reports key off the row id),
-- so dropping the constraint is safe.
--
-- Forward-only; the down migration exists for local-dev convenience.

BEGIN;

ALTER TABLE mcp_servers DROP CONSTRAINT mcp_servers_publisher_id_slug_key;
ALTER TABLE agents      DROP CONSTRAINT agents_publisher_id_slug_key;

CREATE UNIQUE INDEX mcp_servers_publisher_slug_live
    ON mcp_servers (publisher_id, slug) WHERE status != 'deleted';
CREATE UNIQUE INDEX agents_publisher_slug_live
    ON agents (publisher_id, slug) WHERE status != 'deleted';

COMMIT;
