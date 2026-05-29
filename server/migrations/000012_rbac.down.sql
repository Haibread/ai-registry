-- 000012_rbac.down.sql
-- Reverses 000012_rbac.up.sql. For local-dev convenience only — never relied
-- on in production (forward-only migrations, see CLAUDE.md).
--
-- Drops the re-added publisher_id columns and the RBAC tables. workspace_id
-- and the workspaces table are untouched (this migration never removed them).

BEGIN;

DROP INDEX IF EXISTS idx_agents_publisher;
DROP INDEX IF EXISTS idx_mcp_servers_publisher;

ALTER TABLE agents      DROP COLUMN IF EXISTS publisher_id;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS publisher_id;

DROP TABLE IF EXISTS role_grants;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS users;

COMMIT;
