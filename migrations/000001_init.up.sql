-- Fluxa's whole schema, in one migration.
--
-- The project has never been released, so there is no deployed database
-- whose history has to be replayed: the incremental migrations this
-- replaces only ever ran against development databases. Squashing them
-- keeps the schema readable as a single description of what the system
-- stores, rather than a diff log that has to be applied in the head to
-- work out the current shape.
--
-- All money columns are integer minor units (fen/cents), never floats.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ── User module ──────────────────────────────────────────────────────
-- Organization, departments, members, RBAC, identity sources, local
-- accounts, sessions, notify channels.

-- Exactly one row per deployment: each deployment serves a single
-- company (see DESIGN.md section 9, no cross-tenant isolation needed).
CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name         text NOT NULL,
    is_builtin   boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

-- Static reference data, seeded at the bottom of this file. A permission
-- code is a stable string handlers check against (e.g.
-- "provider.manage_credentials"); see internal/rbac.
CREATE TABLE permissions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text NOT NULL UNIQUE,
    description text NOT NULL
);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE departments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name           text NOT NULL,
    -- Nullable: a department may not have a lead assigned yet, in which
    -- case quota-request approval falls back to an admin (see
    -- DESIGN.md 8.4).
    lead_member_id uuid,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE members (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    -- Nullable: an employee can exist without a department assignment.
    department_id uuid REFERENCES departments (id) ON DELETE SET NULL,
    role_id       uuid NOT NULL REFERENCES roles (id),
    name          text NOT NULL,
    email         text,
    phone         text,
    status        text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'pending_review', 'disabled')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- departments and members reference each other, so the lead FK can only
-- be added once both tables exist.
ALTER TABLE departments
    ADD CONSTRAINT departments_lead_member_fk
    FOREIGN KEY (lead_member_id) REFERENCES members (id) ON DELETE SET NULL;

-- External IM identity bound to a member (Feishu, WeCom, DingTalk).
CREATE TABLE external_identities (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id        uuid NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    provider         text NOT NULL CHECK (provider IN ('feishu', 'wecom', 'dingtalk')),
    external_user_id text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_user_id)
);

-- Admin-managed OAuth app credentials per identity provider, not baked
-- into config files (DESIGN.md 7.1).
CREATE TABLE identity_configs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider      text NOT NULL UNIQUE CHECK (provider IN ('feishu', 'wecom', 'dingtalk')),
    app_id        text NOT NULL DEFAULT '',
    app_secret    text NOT NULL DEFAULT '',
    callback_path text NOT NULL DEFAULT '',
    enabled       boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Singleton settings row for the local-account fallback (phone/email
-- self-registration for companies without a unified IM).
CREATE TABLE auth_settings (
    id                              boolean PRIMARY KEY DEFAULT true CHECK (id),
    local_account_enabled           boolean NOT NULL DEFAULT false,
    local_account_requires_approval boolean NOT NULL DEFAULT true
);
INSERT INTO auth_settings (id) VALUES (true);

-- Local accounts authenticate with a one-time code sent to phone/email
-- (see local_otp_codes below), not a password: matching the login
-- screen design, there is no password field anywhere in the product.
CREATE TABLE local_accounts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id  uuid NOT NULL UNIQUE REFERENCES members (id) ON DELETE CASCADE,
    phone      text UNIQUE,
    email      text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (phone IS NOT NULL OR email IS NOT NULL)
);

-- One-time codes for both registration (proving you own the phone/email
-- before a pending member is created) and login. code_hash is
-- sha256(code): short-lived and single-use, so slow hashing buys
-- nothing a tight expiry doesn't already cover.
CREATE TABLE local_otp_codes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identifier  text NOT NULL,
    purpose     text NOT NULL CHECK (purpose IN ('register', 'login')),
    code_hash   text NOT NULL,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX local_otp_codes_identifier_idx ON local_otp_codes (identifier, purpose);

-- Server-side session (DESIGN.md 7.1 "会话机制"): the client only ever
-- holds an opaque token, and we store a hash of it here, never the raw
-- value, the same way virtual keys are hashed in the provider module.
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    member_id  uuid NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX sessions_member_id_idx ON sessions (member_id);

-- Pluggable SMS/email sending channel used for local-account OTP codes
-- (DESIGN.md 7.1). config holds provider-specific credentials
-- (AccessKey/secret/sign/template, or SMTP host/port/user/pass).
CREATE TABLE notify_channels (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       text NOT NULL UNIQUE CHECK (kind IN ('sms', 'email')),
    provider   text NOT NULL DEFAULT '',
    config     jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled    boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notify_log (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       text NOT NULL CHECK (kind IN ('sms', 'email')),
    recipient  text NOT NULL,
    purpose    text NOT NULL,
    sent_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notify_log_kind_sent_at_idx ON notify_log (kind, sent_at);

-- ── Provider module ──────────────────────────────────────────────────
-- Providers, models & pricing, procurement ledger, routing rules,
-- virtual keys, department quota pools, health state.

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

-- ── Security module ──────────────────────────────────────────────────
-- DLP rules and the events they produce. v1 only ever scans outbound
-- request content, never the model's response (see DESIGN.md 7.3 for
-- why: it would break the "never buffer a streaming response" gateway
-- performance principle).

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

-- ── Audit module ─────────────────────────────────────────────────────
-- Per-request call logs plus admin operation audit, distinct from each
-- other (DESIGN.md v2 note: "操作审计日志...区别于调用日志").

-- member_id, provider_id and model_id are nullable on purpose. Requiring
-- them looked like a guarantee and was really two bugs:
--
-- 1. A department-owned virtual key has no owning member, so the gateway
--    had no value to write and every such insert failed on `invalid
--    input syntax for type uuid` -- calls billed to a department went
--    silently unlogged, successful ones included.
-- 2. A request rejected before routing (revoked key, DLP block, no
--    healthy route, model out of the key's scope) has no provider or
--    model to name, so it could not be recorded at all. Those are
--    exactly the failures an operator opens 调用日志 to explain.
--
-- Nullable is the honest shape: each column means "which member /
-- provider / model, where one applies". The foreign keys stay, so a
-- non-null value still has to reference a real row.
CREATE TABLE call_logs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id      uuid REFERENCES members (id),
    virtual_key_id uuid NOT NULL REFERENCES virtual_keys (id),
    provider_id    uuid REFERENCES providers (id),
    model_id       uuid REFERENCES models (id),
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

-- ── Seed data ────────────────────────────────────────────────────────

-- The fixed set of permission points (DESIGN.md 7.1 "权限点"). This is
-- global reference data with no organization dependency, so it is safe
-- to seed at migration time. Built-in roles are per-organization
-- (roles.org_id is NOT NULL) and are created when an organization is
-- bootstrapped -- see internal/user/service.go EnsureBuiltinRoles, not
-- here.
--
-- Keep this list in sync with internal/rbac/permission.go: it is the
-- single source of truth for which codes exist.
INSERT INTO permissions (code, description) VALUES
    ('provider.manage_credentials',   'Manage provider credentials, models and pricing'),
    ('provider.view',                 'View providers and models'),
    ('provider.manage_routing',       'Manage global routing rules'),
    ('provider.record_procurement',   'Record a procurement (top-up) entry'),
    ('provider.use_playground',       'Use the model playground'),
    ('org.manage_members',            'Manage members: add, edit, approve, reassign department'),
    ('org.manage_departments',        'Manage departments'),
    ('org.manage_roles',              'Manage roles and their permission grants'),
    ('org.manage_identity_sources',   'Manage identity source (SSO) configuration'),
    ('org.manage_notify_channels',    'Manage SMS/email sending channel configuration'),
    ('org.manage_keys',               'Create and revoke virtual keys beyond one''s own'),
    ('org.view_own_usage',            'View one''s own usage and pricing'),
    ('org.manage_personal_routing',   'Configure one''s own personal routing rules'),
    ('org.request_quota',             'Submit a quota request'),
    ('org.approve_department_quota',  'Approve quota requests within one''s own department'),
    ('quota.adjust_any_member',       'Directly adjust any member''s quota'),
    ('quota.approve_any',             'Approve any quota request as an admin fallback'),
    ('security.manage_dlp_rules',     'Manage DLP rules'),
    ('security.view_events',          'View security events'),
    ('audit.view_call_logs',          'View call logs'),
    ('audit.view_operation_logs',     'View operation audit logs');

-- The built-in DLP rules shown in the design mockup: pattern holds a
-- discriminator ('id_card' / 'bank_card') for the checksummed built-in
-- detectors in internal/security/rules, not a literal regex -- see
-- internal/security/service.go findMatches.
--
-- Only these two. An earlier revision also seeded 高风险关键词 as an
-- enabled `block` rule with an empty pattern; the matcher skips empty
-- keywords, so it never blocked anything and merely sat in the table
-- looking like an active line of defence.
INSERT INTO dlp_rules (name, match_type, pattern, action, priority, enabled) VALUES
    ('身份证号识别', 'regex_checksum', 'id_card', 'mask', 10, true),
    ('银行卡号识别', 'regex_checksum', 'bank_card', 'mask', 20, true);
