-- Add status_message and status_changed_at to version lifecycle tracking.
-- Add 'deleted' to mcp_servers status enum.
ALTER TABLE mcp_server_versions
    ADD COLUMN IF NOT EXISTS status_message TEXT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Allow servers to be marked as deleted (all versions deleted).
ALTER TABLE mcp_servers DROP CONSTRAINT IF EXISTS mcp_servers_status_check;
ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_status_check
    CHECK (status IN ('draft', 'published', 'deprecated', 'deleted'));
