ALTER TABLE mcp_server_versions
DROP CONSTRAINT mcp_server_versions_protocol_versions_not_empty;

ALTER TABLE mcp_server_versions
RENAME COLUMN protocol_versions TO protocol_version;

ALTER TABLE mcp_server_versions
ALTER COLUMN protocol_version TYPE TEXT USING protocol_version[1];
