-- The registry catalogs MCP servers only. Rows in the shared tables that
-- point at an agent have no target left once the agent tables go, and would
-- carry a resource type the API no longer defines.
DELETE FROM entry_change_requests
WHERE resource_type = 'agent';

DELETE FROM reports
WHERE resource_type = 'agent';

DELETE FROM audit_log
WHERE resource_type = 'agent' OR action LIKE 'agent%';

ALTER TABLE entry_change_requests
DROP CONSTRAINT ecr_resource_type_chk,
ADD CONSTRAINT ecr_resource_type_chk CHECK (resource_type = 'mcp_server');

ALTER TABLE reports
DROP CONSTRAINT reports_resource_type_check,
ADD CONSTRAINT reports_resource_type_check CHECK (resource_type = 'mcp_server');

DROP TABLE agent_versions;
DROP TABLE agents;
