-- 000014_sessions.up.sql
-- Registry sessions behind an HttpOnly cookie
-- (BFF). Both front doors — brokered OIDC and local password — create a session
-- row. The opaque cookie token is never stored; only its SHA-256 hash
-- (token_hash) is the lookup key, so a DB leak yields no usable session.
-- claim_groups and claim_admin snapshot the OIDC claim inputs at login (empty /
-- false for local logins); effective roles are still resolved live from
-- role_grants on every request.

BEGIN;

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,                       -- ULID, internal
    token_hash   TEXT NOT NULL UNIQUE,                   -- hex SHA-256 of the cookie token
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    auth_method  TEXT NOT NULL CHECK (auth_method IN ('oidc', 'local')),
    claim_groups TEXT[] NOT NULL DEFAULT '{}',           -- snapshotted OIDC claim group slugs
    claim_admin  BOOLEAN NOT NULL DEFAULT false,         -- snapshotted claim Server-Admin
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ                             -- NULL until logout / revoke
);

CREATE INDEX idx_sessions_user ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

COMMIT;
