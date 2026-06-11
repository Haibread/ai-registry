-- 000023_managed_instance_tags.down.sql
ALTER TABLE instance_tags DROP COLUMN IF EXISTS managed;
