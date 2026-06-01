-- 000012_rbac.up.sql
-- Publisher-scoped RBAC + local accounts (additive migration).
--
-- This is the ADDITIVE half of a two-step rollout. The finalising migration
-- (000013) flips publisher_id NOT NULL, restores UNIQUE(publisher_id, slug),
-- and drops workspace_id + the workspaces table.
--
-- This migration:
--   1. Creates the identity + authorization tables (users, groups,
--      group_members, role_grants).
--   2. Re-adds a nullable publisher_id to mcp_servers and agents and backfills
--      it from workspaces.publisher_id via the still-present workspace_id join
--      (the workspace link survives until 000013).
--   3. Converts each workspace group_name binding into a group +
--      Editor grant on the workspace's publisher, so prior write access is
--      preserved without any user-membership migration.
--
-- Until 000013 lands, both publisher_id and workspace_id coexist on resources
-- and new code MUST keep writing workspace_id (it is still NOT NULL).
--
-- The bootstrap admin and the reviewer-group's global grant are seeded on boot
-- (Go-side, idempotent / re-applied) rather than here — see the boot seed.

BEGIN;

-- ── Identity: local + federated principals ────────────────────────────────
-- A principal is a users row keyed by id (a ULID), NOT by the OIDC subject.
-- subject is set for federated logins, password_hash for local logins; a row
-- may carry both (linked), or neither (invited — can hold grants but cannot
-- log in until a credential is set). email is the human key and is normalised
-- to lower-case by the store before insert/lookup.
CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    display_name    TEXT,
    subject         TEXT UNIQUE,                 -- OIDC `sub`; NULL for local-only
    password_hash   TEXT,                        -- argon2id; NULL for OIDC-only
    is_server_admin BOOLEAN NOT NULL DEFAULT false,
    disabled        BOOLEAN NOT NULL DEFAULT false,
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── Groups ("teams") ──────────────────────────────────────────────────────
-- A group's slug is matched verbatim against OIDC claim group names, so an
-- existing Keycloak-group user keeps access once a group of the same slug
-- carries the grant.
CREATE TABLE groups (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_groups_updated_at
    BEFORE UPDATE ON groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE group_members (
    group_id   TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user ON group_members (user_id);

-- ── Role grants ───────────────────────────────────────────────────────────
-- A grant says "principal P holds role R on publisher Q" — or on ALL
-- publishers when publisher_id is NULL. principal_id is polymorphic (a
-- users.id or groups.id) so it carries no FK; the store deletes a principal's
-- grants when its user/group is removed. NULLS NOT DISTINCT (PG15+) makes a
-- global grant collide with itself so duplicates are rejected without a
-- sentinel publisher_id.
CREATE TABLE role_grants (
    id             TEXT PRIMARY KEY,
    principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'group')),
    principal_id   TEXT NOT NULL,
    publisher_id   TEXT REFERENCES publishers(id) ON DELETE CASCADE,  -- NULL = all publishers
    role           TEXT NOT NULL CHECK (role IN ('viewer', 'editor', 'reviewer', 'admin')),
    source         TEXT NOT NULL DEFAULT 'api' CHECK (source IN ('api', 'config')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (principal_type, principal_id, publisher_id, role)
);

CREATE INDEX idx_role_grants_principal ON role_grants (principal_type, principal_id);
CREATE INDEX idx_role_grants_publisher ON role_grants (publisher_id);

-- ── Re-add publisher_id to resources (additive) ───────────────────────────
-- Nullable for now; 000013 backfill-gates and flips it NOT NULL. Resources
-- reach their publisher directly again, mirroring the original pre-workspace shape.
ALTER TABLE mcp_servers
    ADD COLUMN publisher_id TEXT REFERENCES publishers(id) ON DELETE RESTRICT;
ALTER TABLE agents
    ADD COLUMN publisher_id TEXT REFERENCES publishers(id) ON DELETE RESTRICT;

-- Backfill from the workspace → publisher link 000011 left intact.
UPDATE mcp_servers s
    SET publisher_id = w.publisher_id
    FROM workspaces w
    WHERE w.id = s.workspace_id;
UPDATE agents a
    SET publisher_id = w.publisher_id
    FROM workspaces w
    WHERE w.id = a.workspace_id;

CREATE INDEX idx_mcp_servers_publisher ON mcp_servers (publisher_id);
CREATE INDEX idx_agents_publisher      ON agents (publisher_id);

-- ── Convert workspace group bindings → groups + Editor grants ─────────────
-- One-time data migration (this file runs exactly once, so a grant deleted
-- later via the API is never resurrected). Each workspace with a group_name
-- binding becomes a group (slug = the name) plus an Editor grant for that
-- group on the workspace's publisher. The OIDC claim still names the group;
-- the group now carries the grant, so existing Keycloak-group writers keep
-- access with no membership migration. IDs use gen_random_uuid()
-- because SQL has no ULID minting — the id column is opaque TEXT and nothing
-- parses these as ULIDs.
INSERT INTO groups (id, slug, name)
SELECT gen_random_uuid()::text, group_name, group_name
FROM (SELECT DISTINCT group_name FROM workspaces WHERE group_name IS NOT NULL) g
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_grants (id, principal_type, principal_id, publisher_id, role, source)
SELECT gen_random_uuid()::text, 'group', gr.id, w.publisher_id, 'editor', 'api'
FROM (SELECT DISTINCT group_name, publisher_id FROM workspaces WHERE group_name IS NOT NULL) w
JOIN groups gr ON gr.slug = w.group_name
ON CONFLICT (principal_type, principal_id, publisher_id, role) DO NOTHING;

COMMIT;
