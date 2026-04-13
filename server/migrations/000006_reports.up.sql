-- Community-submitted issue reports.
CREATE TABLE reports (
    id            TEXT PRIMARY KEY,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('mcp_server', 'agent')),
    resource_id   TEXT NOT NULL,
    issue_type    TEXT NOT NULL,
    description   TEXT NOT NULL,
    reporter_ip   TEXT,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewed', 'dismissed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at   TIMESTAMPTZ,
    reviewed_by   TEXT
);

CREATE INDEX reports_status_created_at_idx ON reports (status, created_at DESC);
CREATE INDEX reports_resource_idx          ON reports (resource_type, resource_id);
