// Package rbac defines the permission points every role is built from
// and the middleware that enforces them. Roles themselves (built-in and
// custom) are owned by the user module; this package only knows about
// the fixed vocabulary of permissions and how to check one against a
// context that already carries an authenticated member.
package rbac

// Permission is one independently grantable capability. Keep this list
// in sync with migrations/000005_seed_permissions.up.sql -- it is the
// single source of truth for which codes exist, and the seed migration
// mirrors it for storage.
type Permission string

const (
	PermissionProviderManageCredentials Permission = "provider.manage_credentials"
	PermissionProviderView              Permission = "provider.view"
	PermissionProviderManageRouting     Permission = "provider.manage_routing"
	PermissionProviderRecordProcurement Permission = "provider.record_procurement"
	PermissionProviderUsePlayground     Permission = "provider.use_playground"

	PermissionOrgManageMembers          Permission = "org.manage_members"
	PermissionOrgManageDepartments      Permission = "org.manage_departments"
	PermissionOrgManageRoles            Permission = "org.manage_roles"
	PermissionOrgManageIdentitySources  Permission = "org.manage_identity_sources"
	PermissionOrgManageNotifyChannels   Permission = "org.manage_notify_channels"
	PermissionOrgManageKeys             Permission = "org.manage_keys"
	PermissionOrgViewOwnUsage           Permission = "org.view_own_usage"
	PermissionOrgManagePersonalRouting  Permission = "org.manage_personal_routing"
	PermissionOrgRequestQuota           Permission = "org.request_quota"
	PermissionOrgApproveDepartmentQuota Permission = "org.approve_department_quota"

	PermissionQuotaAdjustAnyMember Permission = "quota.adjust_any_member"
	PermissionQuotaApproveAny      Permission = "quota.approve_any"

	PermissionSecurityManageDLPRules Permission = "security.manage_dlp_rules"
	PermissionSecurityViewEvents     Permission = "security.view_events"

	PermissionAuditViewCallLogs      Permission = "audit.view_call_logs"
	PermissionAuditViewOperationLogs Permission = "audit.view_operation_logs"
)
