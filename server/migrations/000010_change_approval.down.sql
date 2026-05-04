-- 000010_change_approval.down.sql

-- Entry tables.
DROP INDEX IF EXISTS agents_active_idx;
ALTER TABLE agents
    DROP COLUMN IF EXISTS deletion_requested_by_email,
    DROP COLUMN IF EXISTS deletion_requested_by,
    DROP COLUMN IF EXISTS deletion_requested_at,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS mcp_servers_active_idx;
ALTER TABLE mcp_servers
    DROP COLUMN IF EXISTS deletion_requested_by_email,
    DROP COLUMN IF EXISTS deletion_requested_by,
    DROP COLUMN IF EXISTS deletion_requested_at,
    DROP COLUMN IF EXISTS deleted_at;

-- Agent versions.
DROP INDEX IF EXISTS agent_versions_one_pending_idx;
DROP INDEX IF EXISTS agent_versions_review_state_idx;
ALTER TABLE agent_versions
    DROP CONSTRAINT IF EXISTS agent_versions_review_unpublished_chk,
    DROP CONSTRAINT IF EXISTS agent_versions_review_state_chk,
    DROP COLUMN IF EXISTS rejection_reason,
    DROP COLUMN IF EXISTS review_decision,
    DROP COLUMN IF EXISTS reviewed_by_email,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS submitted_by_email,
    DROP COLUMN IF EXISTS submitted_by,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS revision,
    DROP COLUMN IF EXISTS review_state;

-- MCP server versions.
DROP INDEX IF EXISTS mcp_server_versions_one_pending_idx;
DROP INDEX IF EXISTS mcp_server_versions_review_state_idx;
ALTER TABLE mcp_server_versions
    DROP CONSTRAINT IF EXISTS mcp_server_versions_review_unpublished_chk,
    DROP CONSTRAINT IF EXISTS mcp_server_versions_review_state_chk,
    DROP COLUMN IF EXISTS rejection_reason,
    DROP COLUMN IF EXISTS review_decision,
    DROP COLUMN IF EXISTS reviewed_by_email,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS submitted_by_email,
    DROP COLUMN IF EXISTS submitted_by,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS revision,
    DROP COLUMN IF EXISTS review_state;
