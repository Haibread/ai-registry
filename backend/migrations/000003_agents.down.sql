-- 000003_agents.down.sql

DROP INDEX IF EXISTS idx_agent_versions_published;
DROP INDEX IF EXISTS idx_agent_versions_agent;
DROP TABLE IF EXISTS agent_versions;

DROP INDEX IF EXISTS idx_agents_search;
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_visibility;
DROP INDEX IF EXISTS idx_agents_publisher;
DROP TABLE IF EXISTS agents;
