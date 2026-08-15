-- Audit module: per-request call logs plus admin operation audit,
-- distinct from each other (DESIGN.md v2 note: "操作审计日志...区别于
-- 调用日志").

CREATE TABLE call_logs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id      uuid NOT NULL REFERENCES members (id),
    virtual_key_id uuid NOT NULL REFERENCES virtual_keys (id),
    provider_id    uuid NOT NULL REFERENCES providers (id),
    model_id       uuid NOT NULL REFERENCES models (id),
    request_id     text NOT NULL,
    status         text NOT NULL CHECK (status IN ('success', 'failed')),
    latency_ms     integer NOT NULL DEFAULT 0,
    input_tokens   integer NOT NULL DEFAULT 0,
    output_tokens  integer NOT NULL DEFAULT 0,
    cost_cents     bigint NOT NULL DEFAULT 0,
    occurred_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX call_logs_member_occurred_idx ON call_logs (member_id, occurred_at DESC);
CREATE INDEX call_logs_occurred_idx ON call_logs (occurred_at DESC);

CREATE TABLE operation_audit_logs (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_member_id   uuid NOT NULL REFERENCES members (id),
    action            text NOT NULL,
    detail            text NOT NULL DEFAULT '',
    occurred_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX operation_audit_logs_occurred_idx ON operation_audit_logs (occurred_at DESC);
