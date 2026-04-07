-- 000002_mcp.down.sql

DROP INDEX IF EXISTS idx_mcp_versions_published;
DROP INDEX IF EXISTS idx_mcp_versions_server;
DROP TABLE IF EXISTS mcp_server_versions;

DROP INDEX IF EXISTS idx_mcp_servers_search;
DROP INDEX IF EXISTS idx_mcp_servers_status;
DROP INDEX IF EXISTS idx_mcp_servers_visibility;
DROP INDEX IF EXISTS idx_mcp_servers_publisher;
DROP TABLE IF EXISTS mcp_servers;
