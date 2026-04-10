-- 000001_init.down.sql
-- Tears down the full schema. Only useful during local development.

DROP TABLE IF EXISTS audit_log;

DROP TRIGGER IF EXISTS trg_agent_versions_updated_at ON agent_versions;
DROP TABLE IF EXISTS agent_versions;
DROP TRIGGER IF EXISTS trg_agents_updated_at ON agents;
DROP TABLE IF EXISTS agents;

DROP TRIGGER IF EXISTS trg_mcp_server_versions_updated_at ON mcp_server_versions;
DROP TABLE IF EXISTS mcp_server_versions;
DROP TRIGGER IF EXISTS trg_mcp_servers_updated_at ON mcp_servers;
DROP TABLE IF EXISTS mcp_servers;

DROP TRIGGER IF EXISTS trg_publishers_updated_at ON publishers;
DROP TABLE IF EXISTS publishers;

DROP FUNCTION IF EXISTS set_updated_at;
