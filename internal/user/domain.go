// Package user implements the User module from DESIGN.md 7.1:
// organization/department/member structure, RBAC roles, identity
// sources, the local-account fallback, and the server-side session that
// keeps a browser logged in.
package user

import "time"

type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type Department struct {
	ID           string
	OrgID        string
	Name         string
	LeadMemberID *string
	CreatedAt    time.Time
}

type MemberStatus string

const (
	MemberStatusActive        MemberStatus = "active"
	MemberStatusPendingReview MemberStatus = "pending_review"
	MemberStatusDisabled      MemberStatus = "disabled"
)

type Member struct {
	ID           string
	OrgID        string
	DepartmentID *string
	RoleID       string
	Name         string
	Email        *string
	Phone        *string
	Status       MemberStatus
	CreatedAt    time.Time
}

type Role struct {
	ID        string
	OrgID     string
	Name      string
	IsBuiltin bool
	CreatedAt time.Time
}

// Built-in role names, created for every organization by
// EnsureBuiltinRoles. Companies can add custom roles on top of these
// (DESIGN.md 7.1: "支持管理员自定义角色...不锁死在内置角色里").
const (
	RoleSuperAdmin     = "超管"
	RoleAdmin          = "管理员"
	RoleDepartmentLead = "部门负责人"
	RoleEmployee       = "员工"
)

type IdentityProvider string

const (
	IdentityProviderFeishu   IdentityProvider = "feishu"
	IdentityProviderWeCom    IdentityProvider = "wecom"
	IdentityProviderDingTalk IdentityProvider = "dingtalk"
)

type ExternalIdentity struct {
	ID             string
	MemberID       string
	Provider       IdentityProvider
	ExternalUserID string
	CreatedAt      time.Time
}

// IdentityConfig holds the OAuth app credentials an admin configures for
// one identity provider, instead of baking them into config files
// (DESIGN.md 7.1).
type IdentityConfig struct {
	ID           string
	Provider     IdentityProvider
	AppID        string
	AppSecret    string
	CallbackPath string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AuthSettings struct {
	LocalAccountEnabled          bool
	LocalAccountRequiresApproval bool
}

type LocalAccount struct {
	ID        string
	MemberID  string
	Phone     *string
	Email     *string
	CreatedAt time.Time
}

type OTPPurpose string

const (
	OTPPurposeRegister OTPPurpose = "register"
	OTPPurposeLogin    OTPPurpose = "login"
)

type NotifyChannelKind string

const (
	NotifyChannelSMS   NotifyChannelKind = "sms"
	NotifyChannelEmail NotifyChannelKind = "email"
)

// NotifyChannel is a pluggable sending channel (DESIGN.md 7.1): Config
// holds provider-specific credentials (AccessKey/secret/sign/template
// for SMS, or SMTP host/port/user/pass for email) as opaque JSON so the
// schema never has to change when a new vendor is added.
type NotifyChannel struct {
	ID        string
	Kind      NotifyChannelKind
	Provider  string
	Config    map[string]any
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
