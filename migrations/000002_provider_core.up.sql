-- Provider module: providers, models & pricing, procurement ledger,
-- routing rules, virtual keys, department quota pools, health state.
-- All money columns are integer minor units (fen/cents), never floats.

CREATE TABLE providers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name       text NOT NULL,
    kind       text NOT NULL
               CHECK (kind IN ('openai_compatible', 'anthropic', 'azure_openai', 'gemini', 'bedrock')),
    -- Endpoint/credential config; self-hosted single-tenant deployment,
    -- so plaintext at rest is an accepted v1 tradeoff (see the SQLite
    -- discussion in DESIGN.md for the same reasoning applied here).
    config     jsonb NOT NULL DEFAULT '{}'::jsonb,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE models (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id                 uuid NOT NULL REFERENCES providers (id) ON DELETE CASCADE,
    name                        text NOT NULL,
    model_identifier            text NOT NULL,
    status                      text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    input_price_cents_per_1m    bigint NOT NULL DEFAULT 0,
    output_price_cents_per_1m   bigint NOT NULL DEFAULT 0,
    context_window              integer NOT NULL DEFAULT 0,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);

-- One row per procurement event; the start of the
-- procurement -> distribution -> consumption ledger (DESIGN.md 8.1).
-- v1 intentionally does not constrain this against distributed totals.
CREATE TABLE procurement_records (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id           uuid NOT NULL REFERENCES providers (id),
    amount_cents          bigint NOT NULL CHECK (amount_cents > 0),
    note                  text NOT NULL DEFAULT '',
    recorded_by_member_id uuid NOT NULL REFERENCES members (id),
    recorded_at           timestamptz NOT NULL DEFAULT now()
);

-- Global (admin-owned) and personal (employee-owned) routing rules
-- share one table; scope + owner_member_id tell them apart.
-- cost_ceiling_cents resolves DESIGN.md open question about a cost
-- circuit-breaker on personal fallback chains: when set, the gateway
-- stops trying further fallback hops once the estimated chain cost for
-- the request would exceed it.
CREATE TABLE routing_rules (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope              text NOT NULL CHECK (scope IN ('global', 'personal')),
    owner_member_id    uuid REFERENCES members (id) ON DELETE CASCADE,
    condition_label    text NOT NULL DEFAULT '默认',
    target_model_id    uuid NOT NULL REFERENCES models (id),
    fallback_model_id  uuid REFERENCES models (id),
    cost_ceiling_cents bigint,
    sort_order         integer NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (scope = 'global' OR owner_member_id IS NOT NULL)
);
CREATE INDEX routing_rules_owner_idx ON routing_rules (owner_member_id);

-- Quota carrier (DESIGN.md 7.2): every programmatic call authenticates
-- with one of these. secret_hash is sha256(raw key); secret_prefix is
-- the short, non-secret part shown in the UI for identification.
CREATE TABLE virtual_keys (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                text NOT NULL,
    secret_hash         text NOT NULL UNIQUE,
    secret_prefix       text NOT NULL,
    owner_type          text NOT NULL CHECK (owner_type IN ('member', 'department')),
    owner_member_id     uuid REFERENCES members (id) ON DELETE CASCADE,
    owner_department_id uuid REFERENCES departments (id) ON DELETE CASCADE,
    -- NULL model_scope means every enabled model is allowed.
    model_scope         uuid[],
    budget_cents        bigint NOT NULL CHECK (budget_cents >= 0),
    spent_cents         bigint NOT NULL DEFAULT 0,
    period_started_at   timestamptz NOT NULL DEFAULT date_trunc('month', now()),
    status               text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    revoked_at          timestamptz,
    CHECK (
        (owner_type = 'member' AND owner_member_id IS NOT NULL AND owner_department_id IS NULL) OR
        (owner_type = 'department' AND owner_department_id IS NOT NULL AND owner_member_id IS NULL)
    )
);
CREATE INDEX virtual_keys_owner_member_idx ON virtual_keys (owner_member_id);
CREATE INDEX virtual_keys_owner_department_idx ON virtual_keys (owner_department_id);

-- Department budget pool. Balance is intentionally NOT stored here: it
-- is computed as total_cents minus the sum of budget_cents across the
-- department's active virtual keys, so it can never drift out of sync
-- with the keys that actually draw from it (see DESIGN.md 7.2
-- "鉴权与一致性").
CREATE TABLE department_quota_pools (
    department_id uuid PRIMARY KEY REFERENCES departments (id) ON DELETE CASCADE,
    total_cents    bigint NOT NULL CHECK (total_cents >= 0),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- An employee's request for a new or larger virtual key budget,
-- approved by their department lead or, failing that, an admin
-- (DESIGN.md 8.4).
CREATE TABLE quota_requests (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_by_member_id uuid NOT NULL REFERENCES members (id),
    model_id            uuid REFERENCES models (id),
    amount_cents        bigint NOT NULL CHECK (amount_cents > 0),
    reason              text NOT NULL DEFAULT '',
    status              text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by_member_id uuid REFERENCES members (id),
    decided_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quota_requests_requester_idx ON quota_requests (requested_by_member_id);

-- Circuit breaker state machine, tracked per provider (DESIGN.md 7.2,
-- normal -> circuit_open -> half_open -> normal).
CREATE TABLE provider_health_states (
    provider_id           uuid PRIMARY KEY REFERENCES providers (id) ON DELETE CASCADE,
    state                 text NOT NULL DEFAULT 'normal'
                          CHECK (state IN ('normal', 'circuit_open', 'half_open')),
    consecutive_failures  integer NOT NULL DEFAULT 0,
    opened_at             timestamptz,
    last_probe_at         timestamptz,
    updated_at            timestamptz NOT NULL DEFAULT now()
);
