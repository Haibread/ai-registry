-- Add STORED tsvector generated columns to mcp_servers and agents.
-- These replace the inline to_tsvector() calls in list queries, giving us
-- a GIN index so full-text search scales beyond a few thousand rows.

ALTER TABLE mcp_servers
    ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            coalesce(name, '') || ' ' ||
            coalesce(description, '') || ' ' ||
            coalesce(slug, '')
        )
    ) STORED;

CREATE INDEX mcp_servers_search_idx ON mcp_servers USING GIN(search_vector);

ALTER TABLE agents
    ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            coalesce(name, '') || ' ' ||
            coalesce(description, '') || ' ' ||
            coalesce(slug, '')
        )
    ) STORED;

CREATE INDEX agents_search_idx ON agents USING GIN(search_vector);
