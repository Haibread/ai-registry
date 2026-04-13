ALTER TABLE agents      DROP COLUMN IF EXISTS copy_count;
ALTER TABLE agents      DROP COLUMN IF EXISTS view_count;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS copy_count;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS view_count;
