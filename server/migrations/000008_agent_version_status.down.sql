DROP INDEX IF EXISTS idx_agent_versions_status;

ALTER TABLE agent_versions
    DROP COLUMN IF EXISTS status_changed_at,
    DROP COLUMN IF EXISTS status_message,
    DROP COLUMN IF EXISTS status;
