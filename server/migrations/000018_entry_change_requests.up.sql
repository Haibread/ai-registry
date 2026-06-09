-- 000018_entry_change_requests.up.sql
-- Route entry-level state mutations through the change-approval review queue.
--
-- Until now visibility / deprecate / metadata-edit took effect immediately,
-- bypassing the review queue that already gates version content (submit ->
-- approve/reject) and entry deletion (deletion-request -> approve/reject).
--
-- Rather than add bespoke per-action columns to each entry table (as the
-- deletion flow does), all three actions share ONE generic table: a pending
-- row carries the proposed mutation as a JSONB payload, and the approve path
-- dispatches on (resource_type, action) to apply it. The deletion flow is
-- left untouched; this table mirrors its patterns (denormalised actor audit
-- columns, optimistic-concurrency revision, one-pending guard).
--
-- IDs are ULIDs stored as TEXT, matching every other table in this schema;
-- entry_id has no DB-level FK because it references one of two tables
-- depending on resource_type (resolved in the store, like the handlers do).

CREATE TABLE entry_change_requests (
    id                 TEXT        PRIMARY KEY,
    resource_type      TEXT        NOT NULL,
    entry_id           TEXT        NOT NULL,
    action             TEXT        NOT NULL,
    payload            JSONB       NOT NULL,
    state              TEXT        NOT NULL DEFAULT 'pending_review',
    revision           INTEGER     NOT NULL DEFAULT 1,
    submitted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_by       TEXT,
    submitted_by_email TEXT,
    reviewed_at        TIMESTAMPTZ,
    reviewed_by        TEXT,
    reviewed_by_email  TEXT,
    decision           TEXT,
    rejection_reason   TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ecr_resource_type_chk CHECK (resource_type IN ('mcp_server', 'agent')),
    CONSTRAINT ecr_action_chk        CHECK (action IN ('visibility', 'deprecation', 'metadata_edit')),
    CONSTRAINT ecr_state_chk         CHECK (state IN ('pending_review', 'approved', 'rejected')),
    CONSTRAINT ecr_decision_chk      CHECK (decision IS NULL OR decision IN ('approved', 'rejected'))
);

-- At most one pending change of any action per entry: keeps the reviewer's
-- mental model simple and removes apply-ordering ambiguity between e.g. a
-- queued visibility flip and a queued metadata edit on the same entry.
CREATE UNIQUE INDEX ecr_one_pending_per_entry_idx
    ON entry_change_requests (resource_type, entry_id)
    WHERE state = 'pending_review';

-- Drives the review-queue union branch + its (submitted_at, entry_id) cursor.
CREATE INDEX ecr_queue_idx
    ON entry_change_requests (submitted_at DESC, entry_id DESC)
    WHERE state = 'pending_review';
