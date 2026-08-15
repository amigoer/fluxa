-- User module: organization, departments, members, RBAC, identity
-- sources, local accounts, sessions, notify channels.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

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

-- Static reference data, seeded in 000005_seed_permissions. A permission
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
