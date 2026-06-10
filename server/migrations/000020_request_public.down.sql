-- 000020_request_public.down.sql

ALTER TABLE mcp_server_versions
    DROP COLUMN request_public;

ALTER TABLE agent_versions
    DROP COLUMN request_public;
