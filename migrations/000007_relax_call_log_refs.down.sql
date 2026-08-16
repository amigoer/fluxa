-- Rows written since the up-migration may legitimately have no member,
-- provider or model, and there is no value to invent for them, so the
-- rollback drops them rather than failing the NOT NULL re-add.
DELETE FROM call_logs WHERE member_id IS NULL OR provider_id IS NULL OR model_id IS NULL;

ALTER TABLE call_logs
    ALTER COLUMN member_id SET NOT NULL,
    ALTER COLUMN provider_id SET NOT NULL,
    ALTER COLUMN model_id SET NOT NULL;
