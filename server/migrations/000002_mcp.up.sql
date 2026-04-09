-- 000002_mcp.up.sql
-- MCP server registry tables.

CREATE TABLE IF NOT EXISTS mcp_servers (
    id           TEXT        PRIMARY KEY,              -- ULID
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    homepage_url TEXT,
    repo_url     TEXT,
    license      TEXT,
    visibility   TEXT        NOT NULL DEFAULT 'private'
                             CHECK (visibility IN ('private', 'public')),
    status       TEXT        NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft', 'published', 'deprecated')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_publisher  ON mcp_servers (publisher_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_visibility ON mcp_servers (visibility);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status     ON mcp_servers (status);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_search     ON mcp_servers
    USING gin(to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,'')));

CREATE TABLE IF NOT EXISTS mcp_server_versions (
    id               TEXT        PRIMARY KEY,          -- ULID
    server_id        TEXT        NOT NULL REFERENCES mcp_servers(id) ON DELETE RESTRICT,
    version          TEXT        NOT NULL,             -- semver
    runtime          TEXT        NOT NULL
                                 CHECK (runtime IN ('stdio','http','sse','streamable_http')),
    packages         JSONB       NOT NULL DEFAULT '[]',
    capabilities     JSONB       NOT NULL DEFAULT '{}',
    protocol_version TEXT        NOT NULL,
    checksum         TEXT,
    signature        TEXT,
    published_at     TIMESTAMPTZ,                      -- NULL until published
    released_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (server_id, version)
);

CREATE INDEX IF NOT EXISTS idx_mcp_versions_server     ON mcp_server_versions (server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_versions_published  ON mcp_server_versions (published_at)
    WHERE published_at IS NOT NULL;
