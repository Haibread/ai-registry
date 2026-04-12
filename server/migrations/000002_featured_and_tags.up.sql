-- 000002_featured_and_tags.up.sql
-- Adds featured flag and tags array to mcp_servers and agents
-- for home page featured entries and category browsing.

ALTER TABLE mcp_servers ADD COLUMN featured BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE mcp_servers ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE agents ADD COLUMN featured BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE agents ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_mcp_servers_featured ON mcp_servers (featured) WHERE featured = true;
CREATE INDEX idx_agents_featured ON agents (featured) WHERE featured = true;
CREATE INDEX idx_mcp_servers_tags ON mcp_servers USING GIN (tags);
CREATE INDEX idx_agents_tags ON agents USING GIN (tags);
