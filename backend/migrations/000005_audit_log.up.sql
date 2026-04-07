-- Audit log: immutable record of every admin mutation.
-- Scope: create, publish, deprecate, visibility toggle, permission changes.

CREATE TABLE audit_log (
    id           TEXT        PRIMARY KEY,
    actor_subject TEXT       NOT NULL,           -- Keycloak subject (user UUID)
    actor_email   TEXT       NOT NULL DEFAULT '', -- human-readable identity
    action        TEXT       NOT NULL,            -- e.g. "mcp_server.created"
    resource_type TEXT       NOT NULL,            -- "mcp_server" | "agent" | "publisher"
    resource_id   TEXT       NOT NULL,            -- ULID of the mutated resource
    resource_ns   TEXT       NOT NULL DEFAULT '', -- publisher slug
    resource_slug TEXT       NOT NULL DEFAULT '', -- resource slug
    metadata      JSONB,                          -- action-specific context (version, old/new visibility, …)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast look-ups by actor and by resource
CREATE INDEX audit_log_actor_idx    ON audit_log(actor_subject);
CREATE INDEX audit_log_resource_idx ON audit_log(resource_type, resource_id);
CREATE INDEX audit_log_created_idx  ON audit_log(created_at DESC);
