-- Remote endpoints for hosted MCP servers, mirroring the `remotes` array of
-- the MCP registry server.json: [{"type": "sse"|"http"|"streamable_http",
-- "url": "https://…"}]. Kept separate from `packages` — a remote has no
-- installable artifact, so forcing it through a package entry (registry type,
-- identifier, version) would fabricate fields that don't exist.
ALTER TABLE mcp_server_versions
    ADD COLUMN remotes JSONB NOT NULL DEFAULT '[]'::jsonb;
