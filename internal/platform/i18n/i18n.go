// Package i18n defines the stable message keys the API returns instead of
// hardcoded human-readable text.
//
// The backend never renders UI copy: it returns a Key (and structured
// data where relevant), and the frontend's own i18n dictionary maps that
// key to localized text (see DESIGN.md 6.4). This package is just the
// shared vocabulary both sides agree on, so a key can't drift between a
// handler and the frontend dictionary without a compile error on one
// side.
package i18n

// Key identifies a message the frontend knows how to render in the
// user's language. Handlers set this on API errors instead of writing
// prose.
type Key string

const (
	KeyInvalidCredentials    Key = "auth.invalid_credentials"
	KeyAccountPendingReview  Key = "auth.account_pending_review"
	KeySessionExpired        Key = "auth.session_expired"
	KeyNotifyChannelMissing  Key = "auth.notify_channel_missing"
	KeyPermissionDenied      Key = "rbac.permission_denied"
	KeyQuotaExceeded         Key = "quota.exceeded"
	KeyQuotaRequestNotFound  Key = "quota.request_not_found"
	KeyProviderUnavailable   Key = "provider.unavailable"
	KeyModelNotEnabled       Key = "provider.model_not_enabled"
	KeyVirtualKeyInvalid     Key = "provider.virtual_key_invalid"
	KeyVirtualKeyRevoked     Key = "provider.virtual_key_revoked"
	KeyDepartmentPoolInvalid Key = "provider.department_pool_invalid"
	KeyValidationFailed      Key = "common.validation_failed"
	KeyNotFound              Key = "common.not_found"
	KeyInternalError         Key = "common.internal_error"
)
