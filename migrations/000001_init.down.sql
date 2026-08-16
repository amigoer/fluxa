-- Dropped in reverse dependency order: audit and security reference the
-- provider and user tables, and within the user module departments and
-- members reference each other, so that constraint goes before either
-- table can be dropped.

DROP TABLE IF EXISTS operation_audit_logs;
DROP TABLE IF EXISTS call_logs;

DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS dlp_rules;

DROP TABLE IF EXISTS provider_health_states;
DROP TABLE IF EXISTS quota_requests;
DROP TABLE IF EXISTS department_quota_pools;
DROP TABLE IF EXISTS virtual_keys;
DROP TABLE IF EXISTS routing_rules;
DROP TABLE IF EXISTS procurement_records;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS providers;

DROP TABLE IF EXISTS notify_log;
DROP TABLE IF EXISTS notify_channels;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS local_otp_codes;
DROP TABLE IF EXISTS local_accounts;
DROP TABLE IF EXISTS auth_settings;
DROP TABLE IF EXISTS identity_configs;
DROP TABLE IF EXISTS external_identities;
ALTER TABLE IF EXISTS departments DROP CONSTRAINT IF EXISTS departments_lead_member_fk;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS organizations;
