-- 000008_workspaces.down.sql
-- Reverse of the schema half of the workspace rollout. Drops the
-- workspace_id columns from resource tables first (to release the FK), then
-- drops the workspaces table.
--
-- Per CLAUDE.md, down migrations are for local dev convenience only.

DROP INDEX IF EXISTS idx_agents_workspace;
ALTER TABLE agents       DROP COLUMN IF EXISTS workspace_id;

DROP INDEX IF EXISTS idx_mcp_servers_workspace;
ALTER TABLE mcp_servers  DROP COLUMN IF EXISTS workspace_id;

DROP TRIGGER IF EXISTS trg_workspaces_updated_at ON workspaces;
DROP TABLE IF EXISTS workspaces;
