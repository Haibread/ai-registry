-- 000001_init.up.sql
-- Clean initial schema for AI Registry.
-- Replaces the previous 8 incremental migrations.
--
-- Design notes:
--   - IDs are ULIDs stored as TEXT (sortable, URL-safe, no native PG type needed at this scale).
--   - Server-level status:  draft | published | deprecated | deleted
--   - Version-level status: active | deprecated | deleted  (draft is implicit: published_at IS NULL)
--   - updated_at is maintained automatically by triggers on every table that carries it.
--   - Full-text search uses STORED tsvector generated columns + GIN index (no expression indexes).
--   - No users table: auth is stateless JWT; user data never hits the database.

-- ── Helpers ────────────────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ── Publishers ─────────────────────────────────────────────────────────────

CREATE TABLE publishers (
    id         TEXT        PRIMARY KEY,
    slug       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    contact    TEXT,
    verified   BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_publishers_slug ON publishers (slug);

CREATE TRIGGER trg_publishers_updated_at
    BEFORE UPDATE ON publishers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── MCP servers ────────────────────────────────────────────────────────────

CREATE TABLE mcp_servers (
    id           TEXT        PRIMARY KEY,
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
                             CHECK (status IN ('draft', 'published', 'deprecated', 'deleted')),
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            coalesce(name, '') || ' ' ||
            coalesce(description, '') || ' ' ||
            coalesce(slug, '')
        )
    ) STORED,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE INDEX idx_mcp_servers_publisher   ON mcp_servers (publisher_id);
CREATE INDEX idx_mcp_servers_visibility  ON mcp_servers (visibility);
CREATE INDEX idx_mcp_servers_status      ON mcp_servers (status);
CREATE INDEX idx_mcp_servers_search      ON mcp_servers USING GIN (search_vector);

CREATE TRIGGER trg_mcp_servers_updated_at
    BEFORE UPDATE ON mcp_servers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE mcp_server_versions (
    id               TEXT        PRIMARY KEY,
    server_id        TEXT        NOT NULL REFERENCES mcp_servers(id) ON DELETE RESTRICT,
    version          TEXT        NOT NULL,
    runtime          TEXT        NOT NULL
                                 CHECK (runtime IN ('stdio', 'http', 'sse', 'streamable_http')),
    packages         JSONB       NOT NULL DEFAULT '[]',
    capabilities     JSONB       NOT NULL DEFAULT '{}',
    protocol_version TEXT        NOT NULL,
    checksum         TEXT,
    signature        TEXT,
    status           TEXT        NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('active', 'deprecated', 'deleted')),
    status_message   TEXT,
    status_changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (server_id, version)
);

CREATE INDEX idx_mcp_versions_server     ON mcp_server_versions (server_id);
CREATE INDEX idx_mcp_versions_published  ON mcp_server_versions (published_at)
    WHERE published_at IS NOT NULL;
CREATE INDEX idx_mcp_versions_status     ON mcp_server_versions (status);

CREATE TRIGGER trg_mcp_server_versions_updated_at
    BEFORE UPDATE ON mcp_server_versions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── Agents ─────────────────────────────────────────────────────────────────

CREATE TABLE agents (
    id           TEXT        PRIMARY KEY,
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    visibility   TEXT        NOT NULL DEFAULT 'private'
                             CHECK (visibility IN ('private', 'public')),
    status       TEXT        NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft', 'published', 'deprecated', 'deleted')),
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            coalesce(name, '') || ' ' ||
            coalesce(description, '') || ' ' ||
            coalesce(slug, '')
        )
    ) STORED,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE INDEX idx_agents_publisher  ON agents (publisher_id);
CREATE INDEX idx_agents_visibility ON agents (visibility);
CREATE INDEX idx_agents_status     ON agents (status);
CREATE INDEX idx_agents_search     ON agents USING GIN (search_vector);

CREATE TRIGGER trg_agents_updated_at
    BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE agent_versions (
    id                   TEXT        PRIMARY KEY,
    agent_id             TEXT        NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    version              TEXT        NOT NULL,
    endpoint_url         TEXT        NOT NULL,
    skills               JSONB       NOT NULL DEFAULT '[]',
    capabilities         JSONB       NOT NULL DEFAULT '{}',
    authentication       JSONB       NOT NULL DEFAULT '[]',
    default_input_modes  TEXT[]      NOT NULL DEFAULT '{"text/plain"}',
    default_output_modes TEXT[]      NOT NULL DEFAULT '{"text/plain"}',
    provider             JSONB,
    documentation_url    TEXT,
    icon_url             TEXT,
    protocol_version     TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active', 'deprecated', 'deleted')),
    status_message       TEXT,
    status_changed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent_id, version)
);

CREATE INDEX idx_agent_versions_agent     ON agent_versions (agent_id);
CREATE INDEX idx_agent_versions_published ON agent_versions (published_at)
    WHERE published_at IS NOT NULL;
CREATE INDEX idx_agent_versions_status    ON agent_versions (status);

CREATE TRIGGER trg_agent_versions_updated_at
    BEFORE UPDATE ON agent_versions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── Audit log ──────────────────────────────────────────────────────────────
-- Immutable append-only table. No updated_at; no trigger needed.

CREATE TABLE audit_log (
    id            TEXT        PRIMARY KEY,
    actor_subject TEXT        NOT NULL,
    actor_email   TEXT        NOT NULL DEFAULT '',
    action        TEXT        NOT NULL,
    resource_type TEXT        NOT NULL,
    resource_id   TEXT        NOT NULL,
    resource_ns   TEXT        NOT NULL DEFAULT '',
    resource_slug TEXT        NOT NULL DEFAULT '',
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_actor    ON audit_log (actor_subject);
CREATE INDEX idx_audit_resource ON audit_log (resource_type, resource_id);
CREATE INDEX idx_audit_created  ON audit_log (created_at DESC);
