-- 000016_bearer_auth.up.sql
-- Replace the BFF cookie-session model with bearer JWT authentication.
--
-- The registry now mints its own short-lived access token (an Ed25519 JWT sent
-- in the Authorization header, validated by signature alone — never stored) and
-- a long-lived refresh token (opaque, single-use, rotated). Only the SHA-256
-- hash of a refresh token is persisted, so a DB leak yields no usable token.
--
-- oidc_auth_requests holds the short-lived login-transaction state (replacing
-- the former HttpOnly transaction cookie); auth_handoff_codes delivers the
-- freshly minted tokens to the SPA after the brokered OIDC redirect without ever
-- putting them in a URL.

BEGIN;

DROP TABLE IF EXISTS sessions;

-- Rotating refresh tokens. A successful login issues one; /auth/refresh rotates
-- it (revokes the presented one, issues a successor linked via rotated_from).
-- Presenting an already-rotated token is treated as theft and revokes the
-- whole lineage (see auth.RefreshManager).
CREATE TABLE refresh_tokens (
    id           TEXT PRIMARY KEY,                       -- ULID, internal
    token_hash   TEXT NOT NULL UNIQUE,                   -- hex SHA-256 of the raw refresh token
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    auth_method  TEXT NOT NULL CHECK (auth_method IN ('oidc', 'local')),
    claim_groups TEXT[] NOT NULL DEFAULT '{}',           -- snapshotted OIDC claim group slugs
    claim_admin  BOOLEAN NOT NULL DEFAULT false,         -- snapshotted claim Server-Admin
    id_token     TEXT,                                   -- OIDC id_token, logout hint only (NULL for local)
    rotated_from TEXT REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ                             -- NULL until rotated / logged out / revoked
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens (expires_at);

-- OIDC login-transaction state, keyed by the hash of the `state` value. Consumed
-- (deleted) by the callback. Short TTL — abandoned sign-ins expire on their own.
CREATE TABLE oidc_auth_requests (
    state_hash    TEXT PRIMARY KEY,                      -- hex SHA-256 of the `state` value
    nonce         TEXT NOT NULL,
    code_verifier TEXT NOT NULL,                         -- PKCE verifier
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_oidc_auth_requests_expires ON oidc_auth_requests (expires_at);

-- One-time handoff codes that deliver minted tokens to the SPA after the OIDC
-- redirect. Keyed by the hash of the code; single-use (consumed on exchange);
-- very short TTL.
CREATE TABLE auth_handoff_codes (
    code_hash     TEXT PRIMARY KEY,                      -- hex SHA-256 of the one-time code
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_in    INTEGER NOT NULL,                      -- access token lifetime in seconds
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    consumed_at   TIMESTAMPTZ                            -- NULL until exchanged
);

CREATE INDEX idx_auth_handoff_codes_expires ON auth_handoff_codes (expires_at);

COMMIT;
