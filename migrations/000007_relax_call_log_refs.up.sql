-- call_logs required a member, provider and model on every row, which
-- turned out to be two bugs rather than one guarantee.
--
-- 1. A department-owned virtual key has no owning member, so the gateway
--    passed an empty string for member_id and every insert failed on
--    `invalid input syntax for type uuid`. The error was discarded at the
--    call site, so calls billed to a department were silently absent from
--    the call log -- including successful ones.
--
-- 2. A request rejected before routing (revoked key, DLP block, no
--    healthy route, model out of the key's scope) has no provider or
--    model to name, so it could not be recorded at all. Those are exactly
--    the failures an operator goes to 调用日志 to explain, and the
--    Playground's diagnostics panel looks the row up by request id.
--
-- Nullable is the honest shape: the column means "which member/provider/
-- model, where one applies". The foreign keys stay, so a non-null value
-- is still guaranteed to reference a real row.
ALTER TABLE call_logs
    ALTER COLUMN member_id DROP NOT NULL,
    ALTER COLUMN provider_id DROP NOT NULL,
    ALTER COLUMN model_id DROP NOT NULL;
