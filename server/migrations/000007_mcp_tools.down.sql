-- 000007_mcp_tools.down.sql
-- Reverse of 000007_mcp_tools.up.sql.
--
-- Per CLAUDE.md the down migrations are for local development convenience
-- only, never relied on in production. Dropping the column discards any
-- publisher-declared tools[] data.

ALTER TABLE mcp_server_versions DROP COLUMN IF EXISTS tools;
