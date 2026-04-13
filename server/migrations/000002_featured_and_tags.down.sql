-- 000002_featured_and_tags.down.sql
DROP INDEX IF EXISTS idx_agents_tags;
DROP INDEX IF EXISTS idx_mcp_servers_tags;
DROP INDEX IF EXISTS idx_agents_featured;
DROP INDEX IF EXISTS idx_mcp_servers_featured;

ALTER TABLE agents DROP COLUMN IF EXISTS tags;
ALTER TABLE agents DROP COLUMN IF EXISTS featured;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS tags;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS featured;
