DROP INDEX IF EXISTS agents_search_idx;
ALTER TABLE agents DROP COLUMN IF EXISTS search_vector;

DROP INDEX IF EXISTS mcp_servers_search_idx;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS search_vector;
