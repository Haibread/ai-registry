-- 000001_init.up.sql
-- Initial schema: publishers and users tables.

CREATE TABLE IF NOT EXISTS publishers (
    id         TEXT        PRIMARY KEY,              -- ULID
    slug       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    contact    TEXT,
    verified   BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_publishers_slug ON publishers (slug);

CREATE TABLE IF NOT EXISTS users (
    id         TEXT        PRIMARY KEY,              -- ULID
    subject    TEXT        NOT NULL UNIQUE,          -- OIDC sub claim
    email      TEXT        NOT NULL,
    roles      TEXT[]      NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_subject ON users (subject);
CREATE INDEX IF NOT EXISTS idx_users_email   ON users (email);
