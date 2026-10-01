-- An MCP server can negotiate several protocol revisions, so a version row
-- lists every revision it supports instead of a single one. Converted in
-- place (not ADD + UPDATE) so the updated_at trigger leaves published rows
-- untouched.
ALTER TABLE mcp_server_versions
ALTER COLUMN protocol_version TYPE TEXT[] USING ARRAY[protocol_version];

ALTER TABLE mcp_server_versions
RENAME COLUMN protocol_version TO protocol_versions;

ALTER TABLE mcp_server_versions
ADD CONSTRAINT mcp_server_versions_protocol_versions_not_empty
CHECK (cardinality(protocol_versions) > 0);
