-- Back to cents. Any sub-cent precision recorded while the schema was in
-- micro-cents is truncated away, which is the same loss that made the
-- move necessary; this exists to unblock a rollback, not to be lossless.

UPDATE procurement_records    SET amount_micro_cents       = amount_micro_cents / 10000;
UPDATE routing_rules          SET cost_ceiling_micro_cents = cost_ceiling_micro_cents / 10000
                              WHERE cost_ceiling_micro_cents IS NOT NULL;
UPDATE virtual_keys           SET budget_micro_cents       = budget_micro_cents / 10000,
                                  spent_micro_cents        = spent_micro_cents / 10000;
UPDATE department_quota_pools SET total_micro_cents        = total_micro_cents / 10000;
UPDATE quota_requests         SET amount_micro_cents       = amount_micro_cents / 10000;
UPDATE call_logs              SET cost_micro_cents         = cost_micro_cents / 10000;

ALTER TABLE procurement_records    RENAME COLUMN amount_micro_cents       TO amount_cents;
ALTER TABLE routing_rules          RENAME COLUMN cost_ceiling_micro_cents TO cost_ceiling_cents;
ALTER TABLE virtual_keys           RENAME COLUMN budget_micro_cents       TO budget_cents;
ALTER TABLE virtual_keys           RENAME COLUMN spent_micro_cents        TO spent_cents;
ALTER TABLE department_quota_pools RENAME COLUMN total_micro_cents        TO total_cents;
ALTER TABLE quota_requests         RENAME COLUMN amount_micro_cents       TO amount_cents;
ALTER TABLE call_logs              RENAME COLUMN cost_micro_cents         TO cost_cents;
