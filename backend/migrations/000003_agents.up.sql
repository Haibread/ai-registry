-- 000003_agents.up.sql
-- Agent registry tables (A2A protocol, June 2025 spec).

CREATE TABLE IF NOT EXISTS agents (
    id           TEXT        PRIMARY KEY,              -- ULID
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    visibility   TEXT        NOT NULL DEFAULT 'private'
                             CHECK (visibility IN ('private', 'public')),
    status       TEXT        NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft', 'published', 'deprecated')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_agents_publisher  ON agents (publisher_id);
CREATE INDEX IF NOT EXISTS idx_agents_visibility ON agents (visibility);
CREATE INDEX IF NOT EXISTS idx_agents_status     ON agents (status);
CREATE INDEX IF NOT EXISTS idx_agents_search     ON agents
    USING gin(to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,'')));

CREATE TABLE IF NOT EXISTS agent_versions (
    id                   TEXT        PRIMARY KEY,      -- ULID
    agent_id             TEXT        NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    version              TEXT        NOT NULL,         -- semver
    endpoint_url         TEXT        NOT NULL,         -- A2A base URL
    -- A2A AgentSkill array (id, name, description, tags[], examples[], inputModes[], outputModes[])
    skills               JSONB       NOT NULL DEFAULT '[]',
    -- A2A AgentCapabilities (streaming, pushNotifications, stateTransitionHistory, extendedAgentCard)
    capabilities         JSONB       NOT NULL DEFAULT '{}',
    -- authentication schemes: array of {scheme: "Bearer"|"ApiKey"|"OAuth2"|"OpenIdConnect", ...}
    authentication       JSONB       NOT NULL DEFAULT '[]',
    default_input_modes  TEXT[]      NOT NULL DEFAULT '{"text/plain"}',
    default_output_modes TEXT[]      NOT NULL DEFAULT '{"text/plain"}',
    -- provider: {organization, url}
    provider             JSONB,
    documentation_url    TEXT,
    icon_url             TEXT,
    protocol_version     TEXT        NOT NULL,         -- A2A spec version targeted
    published_at         TIMESTAMPTZ,                  -- NULL until published
    released_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent_id, version)
);

CREATE INDEX IF NOT EXISTS idx_agent_versions_agent     ON agent_versions (agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_versions_published ON agent_versions (published_at)
    WHERE published_at IS NOT NULL;
