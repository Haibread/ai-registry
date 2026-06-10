-- 000019_entry_change_undeprecation.up.sql
-- Allow 'undeprecation' as an entry-change action: republishing a deprecated
-- entry goes through the same review queue as the other entry-level
-- mutations (deprecation was previously one-way — the UI had no path back).

ALTER TABLE entry_change_requests
    DROP CONSTRAINT ecr_action_chk;

ALTER TABLE entry_change_requests
    ADD CONSTRAINT ecr_action_chk
    CHECK (action IN ('visibility', 'deprecation', 'undeprecation', 'metadata_edit'));
