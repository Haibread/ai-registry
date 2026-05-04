-- 000009_workspace_group_binding.down.sql

DROP INDEX IF EXISTS workspaces_group_name_idx;
ALTER TABLE workspaces DROP COLUMN IF EXISTS group_name;
