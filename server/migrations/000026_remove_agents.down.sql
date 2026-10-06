-- Restores the empty agent tables in their 000025 shape so earlier down
-- migrations still apply; the deleted agent rows are not recoverable.
CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    publisher_id TEXT NOT NULL REFERENCES publishers (id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    readme TEXT,
    visibility TEXT NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private', 'public')),
    status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'published', 'deprecated', 'deleted')),
    featured BOOLEAN NOT NULL DEFAULT FALSE,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    view_count INTEGER NOT NULL DEFAULT 0,
    copy_count INTEGER NOT NULL DEFAULT 0,
    search_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector(
            'english',
            coalesce(name, '') || ' ' || coalesce(description, '') || ' '
            || coalesce(slug, '')
        )
    ) STORED,
    deleted_at TIMESTAMPTZ,
    deletion_requested_at TIMESTAMPTZ,
    deletion_requested_by TEXT,
    deletion_requested_by_email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agents_publisher ON agents (publisher_id);
CREATE INDEX idx_agents_visibility ON agents (visibility);
CREATE INDEX idx_agents_status ON agents (status);
CREATE INDEX idx_agents_search ON agents USING gin (search_vector);
CREATE INDEX idx_agents_featured ON agents (featured) WHERE featured = TRUE;
CREATE INDEX agents_active_idx ON agents (id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX agents_publisher_slug_live
ON agents (publisher_id, slug) WHERE status != 'deleted';

CREATE TRIGGER trg_agents_updated_at
BEFORE UPDATE ON agents
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE agent_versions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents (id) ON DELETE RESTRICT,
    version TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    skills JSONB NOT NULL DEFAULT '[]',
    capabilities JSONB NOT NULL DEFAULT '{}',
    authentication JSONB NOT NULL DEFAULT '[]',
    default_input_modes TEXT[] NOT NULL DEFAULT '{text/plain}',
    default_output_modes TEXT[] NOT NULL DEFAULT '{text/plain}',
    provider JSONB,
    documentation_url TEXT,
    icon_url TEXT,
    protocol_version TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'deprecated', 'deleted')),
    status_message TEXT,
    status_changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    review_state TEXT NOT NULL DEFAULT 'none',
    revision INTEGER NOT NULL DEFAULT 1,
    submitted_at TIMESTAMPTZ,
    submitted_by TEXT,
    submitted_by_email TEXT,
    reviewed_at TIMESTAMPTZ,
    reviewed_by TEXT,
    reviewed_by_email TEXT,
    review_decision TEXT,
    rejection_reason TEXT,
    request_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent_id, version),
    CONSTRAINT agent_versions_review_state_chk
    CHECK (review_state IN ('none', 'pending_review', 'rejected')),
    CONSTRAINT agent_versions_review_unpublished_chk
    CHECK (review_state = 'none' OR published_at IS NULL)
);

CREATE INDEX idx_agent_versions_agent ON agent_versions (agent_id);
CREATE INDEX idx_agent_versions_published ON agent_versions (published_at)
WHERE published_at IS NOT NULL;
CREATE INDEX idx_agent_versions_status ON agent_versions (status);
CREATE INDEX idx_agent_versions_tags ON agent_versions USING gin (tags);
CREATE INDEX agent_versions_review_state_idx ON agent_versions (review_state)
WHERE review_state != 'none';
CREATE UNIQUE INDEX agent_versions_one_pending_idx ON agent_versions (agent_id)
WHERE review_state = 'pending_review';

CREATE TRIGGER trg_agent_versions_updated_at
BEFORE UPDATE ON agent_versions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE reports
DROP CONSTRAINT reports_resource_type_check,
ADD CONSTRAINT reports_resource_type_check
CHECK (resource_type IN ('mcp_server', 'agent'));

ALTER TABLE entry_change_requests
DROP CONSTRAINT ecr_resource_type_chk,
ADD CONSTRAINT ecr_resource_type_chk
CHECK (resource_type IN ('mcp_server', 'agent'));
