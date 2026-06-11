-- Tags defined in the server configuration (INSTANCE_TAGS env /
-- `instance_tags` config key) are reconciled into instance_tags at startup
-- and flagged managed. Managed tags are read-only through the HTTP API —
-- configuration is their source of truth; the flag clears when the tag
-- disappears from the configuration (releasing it to admin-UI ownership).
ALTER TABLE instance_tags ADD COLUMN managed BOOLEAN NOT NULL DEFAULT false;
