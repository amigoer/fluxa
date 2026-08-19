-- Money moves from cents to micro-cents (1 cent = 10,000 micro-cents).
--
-- Per-call cost is derived from per-million-token pricing, and computing
-- it in whole cents meant an integer division that truncated toward
-- zero: at a price of 1000 cents per 1M tokens, an 800-token call costs
-- 0.8 cents and was therefore billed as 0. Most ordinary chat traffic --
-- a few hundred to a couple of thousand tokens a call -- landed on
-- exactly that, so spend was systematically undercounted and the budget
-- ceiling it feeds never fired.
--
-- Every amount moves together on purpose. Leaving admin-entered figures
-- (procurement, pools, ceilings) in cents while spend accrued in
-- micro-cents would put two money units either side of the same
-- comparisons, which is how the next accounting bug gets written.
--
-- The two price columns deliberately do NOT move: they are already
-- quoted per million tokens, so they carry six digits of headroom over a
-- single call's cost and lose nothing.

ALTER TABLE procurement_records    RENAME COLUMN amount_cents       TO amount_micro_cents;
ALTER TABLE routing_rules          RENAME COLUMN cost_ceiling_cents TO cost_ceiling_micro_cents;
ALTER TABLE virtual_keys           RENAME COLUMN budget_cents       TO budget_micro_cents;
ALTER TABLE virtual_keys           RENAME COLUMN spent_cents        TO spent_micro_cents;
ALTER TABLE department_quota_pools RENAME COLUMN total_cents        TO total_micro_cents;
ALTER TABLE quota_requests         RENAME COLUMN amount_cents       TO amount_micro_cents;
ALTER TABLE call_logs              RENAME COLUMN cost_cents         TO cost_micro_cents;

UPDATE procurement_records    SET amount_micro_cents       = amount_micro_cents * 10000;
UPDATE routing_rules          SET cost_ceiling_micro_cents = cost_ceiling_micro_cents * 10000
                              WHERE cost_ceiling_micro_cents IS NOT NULL;
UPDATE virtual_keys           SET budget_micro_cents       = budget_micro_cents * 10000,
                                  spent_micro_cents        = spent_micro_cents * 10000;
UPDATE department_quota_pools SET total_micro_cents        = total_micro_cents * 10000;
UPDATE quota_requests         SET amount_micro_cents       = amount_micro_cents * 10000;

-- Historical call costs are left at their stored value scaled up like
-- everything else. They were already rounded to whole cents when they
-- were written -- mostly down to zero -- and no precision that was lost
-- then can be recovered now; scaling keeps them summable against
-- everything else rather than pretending they are accurate.
UPDATE call_logs              SET cost_micro_cents         = cost_micro_cents * 10000;
