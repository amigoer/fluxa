DROP INDEX IF EXISTS notify_log_status_sent_at_idx;
ALTER TABLE notify_log DROP COLUMN error;
ALTER TABLE notify_log DROP COLUMN status;
