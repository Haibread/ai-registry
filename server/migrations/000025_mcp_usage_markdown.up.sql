-- Publisher-written Markdown that replaces the generated usage instructions
-- on the public Usage tab; empty means "show the generated ones".
ALTER TABLE mcp_servers
ADD COLUMN usage_markdown TEXT NOT NULL DEFAULT '';
