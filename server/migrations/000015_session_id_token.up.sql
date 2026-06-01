-- 000015_session_id_token.up.sql
-- ADR 0006 amendment (2026-06-01): persist the OIDC id_token alongside the
-- session so logout can drive RP-initiated logout (id_token_hint) and end the
-- IdP SSO session — without it, "Sign out" only drops the registry cookie while
-- the Keycloak SSO session lives on. NULL for local-password sessions, which
-- have no IdP session to end. It is an identity token (no API authority); it is
-- only ever read server-side as a logout hint.

BEGIN;

ALTER TABLE sessions ADD COLUMN id_token TEXT;

COMMIT;
