-- 000020_request_public.up.sql
-- Author-declared release intent: a version submission may request that the
-- owning entry be made public when the submission is approved ("create and
-- ask for publish"). The flag lives on the version row so it is reviewed,
-- approved, and applied atomically with the version itself — approval flips
-- the entry's visibility in the same transaction. Set on every submit,
-- cleared on withdraw; only consulted while review_state = 'pending_review'.

ALTER TABLE mcp_server_versions
    ADD COLUMN request_public boolean NOT NULL DEFAULT false;

ALTER TABLE agent_versions
    ADD COLUMN request_public boolean NOT NULL DEFAULT false;
