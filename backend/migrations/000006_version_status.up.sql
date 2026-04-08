-- Add per-version lifecycle status to support MCP registry spec
-- PATCH /v0/servers/{name}/versions/{version}/status endpoint.
-- Status enum mirrors the spec: active | deprecated | deleted
ALTER TABLE mcp_server_versions
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deprecated', 'deleted'));
