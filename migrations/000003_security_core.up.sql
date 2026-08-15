-- Security module: DLP rules and the events they produce. v1 only ever
-- scans outbound request content, never the model's response (see
-- DESIGN.md 7.3 for why: it would break the "never buffer a streaming
-- response" gateway performance principle).

CREATE TABLE dlp_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    match_type text NOT NULL CHECK (match_type IN ('regex_checksum', 'keyword')),
    pattern    text NOT NULL,
    action     text NOT NULL CHECK (action IN ('mask', 'block')),
    priority   integer NOT NULL DEFAULT 100,
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE security_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id      uuid REFERENCES members (id),
    virtual_key_id uuid REFERENCES virtual_keys (id),
    rule_id        uuid REFERENCES dlp_rules (id),
    description    text NOT NULL,
    action_taken   text NOT NULL CHECK (action_taken IN ('mask', 'block')),
    occurred_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX security_events_occurred_at_idx ON security_events (occurred_at DESC);
