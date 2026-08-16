-- Outcome of each send attempt.
--
-- The table only ever received rows after a successful send, so the one
-- question an admin actually asks it -- "did anything fail, and why?" --
-- had no answer here. Existing rows are all successes by construction,
-- which is exactly what the default backfills.
ALTER TABLE notify_log ADD COLUMN status text NOT NULL DEFAULT 'sent'
    CHECK (status IN ('sent', 'failed'));

-- The relay's own words for a failure. Null on success.
ALTER TABLE notify_log ADD COLUMN error text;

CREATE INDEX notify_log_status_sent_at_idx ON notify_log (status, sent_at);
