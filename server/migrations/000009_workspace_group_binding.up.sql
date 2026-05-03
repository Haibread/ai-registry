-- 000009_workspace_group_binding.up.sql
-- Phase 7.2 (per ADR 0002): bind each workspace to at most one Keycloak
-- group. NULL means admin-only — preserves the post-0008 behavior for
-- migration safety. Members of the named group can author content for
-- the workspace; admins can author for any workspace.

ALTER TABLE workspaces
    ADD COLUMN group_name TEXT;

CREATE INDEX workspaces_group_name_idx
    ON workspaces (group_name)
    WHERE group_name IS NOT NULL;
