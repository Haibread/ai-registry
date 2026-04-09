ALTER TABLE mcp_server_versions
    DROP COLUMN IF EXISTS status_message,
    DROP COLUMN IF EXISTS status_changed_at;
ALTER TABLE mcp_servers DROP CONSTRAINT IF EXISTS mcp_servers_status_check;
ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_status_check
    CHECK (status IN ('draft', 'published', 'deprecated'));
