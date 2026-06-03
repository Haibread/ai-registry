-- 000016_bearer_auth.down.sql
-- Reverse 000016: drop the bearer-auth tables and recreate the cookie-session
-- table. Local-dev convenience only; never relied on in production.

BEGIN;

DROP TABLE IF EXISTS auth_handoff_codes;
DROP TABLE IF EXISTS oidc_auth_requests;
DROP TABLE IF EXISTS refresh_tokens;

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    token_hash   TEXT NOT NULL UNIQUE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    auth_method  TEXT NOT NULL CHECK (auth_method IN ('oidc', 'local')),
    claim_groups TEXT[] NOT NULL DEFAULT '{}',
    claim_admin  BOOLEAN NOT NULL DEFAULT false,
    id_token     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

COMMIT;
