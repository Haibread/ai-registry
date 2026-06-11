-- 000022_instance_tags.down.sql
ALTER TABLE mcp_servers ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agents      ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_mcp_servers_tags ON mcp_servers USING GIN (tags);
CREATE INDEX idx_agents_tags      ON agents USING GIN (tags);

DROP INDEX IF EXISTS idx_agent_versions_tags;
DROP INDEX IF EXISTS idx_mcp_versions_tags;
ALTER TABLE agent_versions      DROP COLUMN IF EXISTS tags;
ALTER TABLE mcp_server_versions DROP COLUMN IF EXISTS tags;

DROP TABLE IF EXISTS instance_tags;
