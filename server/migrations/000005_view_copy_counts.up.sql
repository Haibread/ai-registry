-- Add view and copy counts to MCP servers and agents.
ALTER TABLE mcp_servers ADD COLUMN view_count  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mcp_servers ADD COLUMN copy_count  INTEGER NOT NULL DEFAULT 0;

ALTER TABLE agents      ADD COLUMN view_count  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents      ADD COLUMN copy_count  INTEGER NOT NULL DEFAULT 0;
