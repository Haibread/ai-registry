-- 000019_entry_change_undeprecation.down.sql
-- Local convenience only. Rows with action='undeprecation' must be removed
-- before the narrower constraint can be restored.

DELETE FROM entry_change_requests WHERE action = 'undeprecation';

ALTER TABLE entry_change_requests
    DROP CONSTRAINT ecr_action_chk;

ALTER TABLE entry_change_requests
    ADD CONSTRAINT ecr_action_chk
    CHECK (action IN ('visibility', 'deprecation', 'metadata_edit'));
