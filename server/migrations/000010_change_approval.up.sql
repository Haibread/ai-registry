-- 000010_change_approval.up.sql
-- Phase 7.3: change-approval workflow.
--
-- Adds an orthogonal `review_state` column to each version table, plus
-- denormalised audit columns recording who submitted / reviewed each
-- version. The existing `status` and `published_at` columns are
-- untouched — the approve handler still flips `published_at` exactly
-- as today, so the public read path is unchanged.
--
-- Also adds deletion-request columns to the entry tables so deletes can
-- pass through the same review queue (publishers request, reviewers
-- approve or reject, soft-delete via `deleted_at`).
--
-- Constraints:
--   - review_state IN ('none','pending_review','rejected')
--   - non-'none' review_state implies published_at IS NULL
--   - at most one 'pending_review' version per entry (partial unique idx)

-- ── MCP server versions ────────────────────────────────────────────────────

ALTER TABLE mcp_server_versions
    ADD COLUMN review_state       TEXT        NOT NULL DEFAULT 'none',
    ADD COLUMN revision           INTEGER     NOT NULL DEFAULT 1,
    ADD COLUMN submitted_at       TIMESTAMPTZ,
    ADD COLUMN submitted_by       TEXT,
    ADD COLUMN submitted_by_email TEXT,
    ADD COLUMN reviewed_at        TIMESTAMPTZ,
    ADD COLUMN reviewed_by        TEXT,
    ADD COLUMN reviewed_by_email  TEXT,
    ADD COLUMN review_decision    TEXT,
    ADD COLUMN rejection_reason   TEXT;

ALTER TABLE mcp_server_versions
    ADD CONSTRAINT mcp_server_versions_review_state_chk
    CHECK (review_state IN ('none','pending_review','rejected'));

ALTER TABLE mcp_server_versions
    ADD CONSTRAINT mcp_server_versions_review_unpublished_chk
    CHECK (review_state = 'none' OR published_at IS NULL);

CREATE INDEX mcp_server_versions_review_state_idx
    ON mcp_server_versions (review_state)
    WHERE review_state != 'none';

CREATE UNIQUE INDEX mcp_server_versions_one_pending_idx
    ON mcp_server_versions (server_id)
    WHERE review_state = 'pending_review';

-- ── Agent versions ─────────────────────────────────────────────────────────

ALTER TABLE agent_versions
    ADD COLUMN review_state       TEXT        NOT NULL DEFAULT 'none',
    ADD COLUMN revision           INTEGER     NOT NULL DEFAULT 1,
    ADD COLUMN submitted_at       TIMESTAMPTZ,
    ADD COLUMN submitted_by       TEXT,
    ADD COLUMN submitted_by_email TEXT,
    ADD COLUMN reviewed_at        TIMESTAMPTZ,
    ADD COLUMN reviewed_by        TEXT,
    ADD COLUMN reviewed_by_email  TEXT,
    ADD COLUMN review_decision    TEXT,
    ADD COLUMN rejection_reason   TEXT;

ALTER TABLE agent_versions
    ADD CONSTRAINT agent_versions_review_state_chk
    CHECK (review_state IN ('none','pending_review','rejected'));

ALTER TABLE agent_versions
    ADD CONSTRAINT agent_versions_review_unpublished_chk
    CHECK (review_state = 'none' OR published_at IS NULL);

CREATE INDEX agent_versions_review_state_idx
    ON agent_versions (review_state)
    WHERE review_state != 'none';

CREATE UNIQUE INDEX agent_versions_one_pending_idx
    ON agent_versions (agent_id)
    WHERE review_state = 'pending_review';

-- ── Pending deletion + soft-delete on entry rows ───────────────────────────

ALTER TABLE mcp_servers
    ADD COLUMN deleted_at                  TIMESTAMPTZ,
    ADD COLUMN deletion_requested_at       TIMESTAMPTZ,
    ADD COLUMN deletion_requested_by       TEXT,
    ADD COLUMN deletion_requested_by_email TEXT;

CREATE INDEX mcp_servers_active_idx
    ON mcp_servers (id)
    WHERE deleted_at IS NULL;

ALTER TABLE agents
    ADD COLUMN deleted_at                  TIMESTAMPTZ,
    ADD COLUMN deletion_requested_at       TIMESTAMPTZ,
    ADD COLUMN deletion_requested_by       TEXT,
    ADD COLUMN deletion_requested_by_email TEXT;

CREATE INDEX agents_active_idx
    ON agents (id)
    WHERE deleted_at IS NULL;
