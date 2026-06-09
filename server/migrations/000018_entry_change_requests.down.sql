-- 000018_entry_change_requests.down.sql
-- Drop the generic entry-change-request channel. Any rows still in
-- 'pending_review' are lost on rollback — acceptable, they were never applied
-- to the underlying entry, and the immediate code paths still exist.

DROP TABLE IF EXISTS entry_change_requests;
