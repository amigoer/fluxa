-- Seeds the fixed set of permission points (DESIGN.md 7.1 "权限点").
-- This is global reference data with no organization dependency, so it
-- is safe to seed at migration time. Built-in roles are per-organization
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
