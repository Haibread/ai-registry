-- Add per-version lifecycle status to agent_versions.
-- Mirrors mcp_server_versions (migrations 000006 + 000007).
ALTER TABLE agent_versions
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deprecated', 'deleted')),
    ADD COLUMN IF NOT EXISTS status_message TEXT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_agent_versions_status ON agent_versions (status);
