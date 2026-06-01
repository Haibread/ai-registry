-- 000015_session_id_token.down.sql
-- Reverse 000015: drop the stored OIDC id_token column.

BEGIN;

ALTER TABLE sessions DROP COLUMN IF EXISTS id_token;

COMMIT;
