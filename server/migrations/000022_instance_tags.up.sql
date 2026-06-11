-- Instance-wide tag vocabulary, curated by Server Admins. Publishers tick
-- tags from this vocabulary when creating a version; the chosen slugs are
-- frozen into the immutable version row. Tag definitions are therefore
-- deactivated (active = false) rather than deleted once referenced by a
-- published version — hard delete is reserved for never-used tags.
CREATE TABLE instance_tags (
    id          TEXT        PRIMARY KEY,
    slug        TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    color       TEXT        NOT NULL DEFAULT '',
    active      BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_instance_tags_updated_at
    BEFORE UPDATE ON instance_tags
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Tags attach to versions (immutable once published), stored as the ticked
-- slugs. Validation against instance_tags happens at create time in the
-- handler; no FK so that deactivated/renamed-display tags never invalidate
-- frozen history.
ALTER TABLE mcp_server_versions ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agent_versions      ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_mcp_versions_tags   ON mcp_server_versions USING GIN (tags);
CREATE INDEX idx_agent_versions_tags ON agent_versions USING GIN (tags);

-- The entry-level tags columns from 000002 were never written by any code
-- path (no endpoint, UI, or seed): always '{}'. Entry-level `tags` in API
-- responses is now derived from the latest published version, so the dead
-- columns go away (their GIN indexes are dropped with them).
ALTER TABLE mcp_servers DROP COLUMN tags;
ALTER TABLE agents      DROP COLUMN tags;
