-- Two-phase quota: reserve before the upstream call, settle after it.
--
-- Spend used to be deducted only once the response had already been
-- streamed to the caller, so the budget check happened after the money
-- was spent. Going over it produced an audit line reading
-- "quota_exceeded_after_call" and nothing else -- there was nothing left
-- to refuse. Concurrent calls on one key could each pass their own
-- post-hoc check and collectively overrun the budget without limit,
-- which is precisely the runaway-agent case the product exists to stop.
--
-- reserved_micro_cents is what is currently promised to in-flight calls.
-- A key's available balance is budget - spent - reserved, and admission
-- is decided against that, so N concurrent calls cannot each be told
-- there is room for the same money.

ALTER TABLE virtual_keys
    ADD COLUMN reserved_micro_cents bigint NOT NULL DEFAULT 0
        CHECK (reserved_micro_cents >= 0);

-- One row per in-flight call. The row is the reservation's identity:
-- settling or releasing deletes it and adjusts the key in the same
-- statement, so a reservation can never be counted twice or stranded
-- without a matching decrement.
--
-- expires_at exists because a process that dies mid-call leaves its
-- reservation behind, and reserved budget that nothing will ever settle
-- would slowly strangle the key. A sweeper releases anything past it.
CREATE TABLE quota_reservations (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    virtual_key_id     uuid NOT NULL REFERENCES virtual_keys (id) ON DELETE CASCADE,
    amount_micro_cents bigint NOT NULL CHECK (amount_micro_cents >= 0),
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL
);
CREATE INDEX quota_reservations_key_idx ON quota_reservations (virtual_key_id);
CREATE INDEX quota_reservations_expires_idx ON quota_reservations (expires_at);
