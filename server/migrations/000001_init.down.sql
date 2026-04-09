-- 000001_init.down.sql

DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_subject;
DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_publishers_slug;
DROP TABLE IF EXISTS publishers;
