package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/ratelimit"
	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/user/identity"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/service"
	"github.com/amigoer/fluxa/internal/user/session"
)

// Handler wires the User module's HTTP surface: setup, login (Feishu +
// local OTP), session-authenticated member/department/role/identity
// config management. Admin and employee views share these same
// endpoints; rbac.Require on each route is what limits what a caller can
// see or do (DESIGN.md 6.3: "不拆成两个独立的 App").
//
// The two route tables live here and nothing else does: each endpoint's
// implementation sits in the file for its feature, next to this one.
type Handler struct {
	service  service.Service
	repo     repo.Repo
	sessions *session.Manager
	feishu   *identity.FeishuAdapter
	baseURL  string

	// Limiters for the OTP endpoints, which are public, take an arbitrary
	// address, and now that the mail channel actually delivers, make the
	// server send real mail (and real, billable SMS) on demand.
	otpCooldown      *ratelimit.Window
	otpPerIdentifier *ratelimit.Window
	otpPerIP         *ratelimit.Window
}

func New(service service.Service, repo repo.Repo, sessions *session.Manager, baseURL string) *Handler {
	return &Handler{
		service:          service,
		repo:             repo,
		sessions:         sessions,
		feishu:           identity.NewFeishuAdapter(),
		baseURL:          baseURL,
		otpCooldown:      ratelimit.NewWindow(1, otpCooldownWindow),
		otpPerIdentifier: ratelimit.NewWindow(otpBurstPerIdentifier, otpBurstWindow),
		otpPerIP:         ratelimit.NewWindow(otpBurstPerIP, otpIPWindow),
	}
}

// RegisterPublicRoutes mounts the endpoints that have to work before any
// session exists: first-run setup, and every login path.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/api/setup/status", h.setupStatus)
	r.Post("/api/setup", h.setup)

	r.Get("/api/auth/methods", h.authMethods)
	r.Get("/api/auth/feishu/login", h.feishuLogin)
	r.Get("/api/auth/feishu/callback", h.feishuCallback)
	r.Post("/api/auth/local/register/request-otp", h.requestRegisterOTP)
	r.Post("/api/auth/local/register/verify", h.verifyRegisterOTP)
	r.Post("/api/auth/local/login/request-otp", h.requestLoginOTP)
	r.Post("/api/auth/local/login/verify", h.verifyLoginOTP)
}

// RegisterProtectedRoutes mounts everything that needs a session. The
// caller passes the group that already carries the session middleware
// (see cmd/server/routes.go) instead of this handler making its own, so
// the User module's endpoints sit behind exactly the same stack as every
// other module's -- including the operation-audit recorder, which is the
// reason the group is shared rather than per-module.
//
// rbac.Require on each route is what limits what a caller can see or do
// (DESIGN.md 6.3: "不拆成两个独立的 App").
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Post("/api/auth/logout", h.logout)
	r.Get("/api/me", h.me)

	r.With(rbac.Require(rbac.PermissionOrgManageMembers)).Get("/api/members", h.listMembers)
	r.With(rbac.Require(rbac.PermissionOrgManageMembers)).Post("/api/members/{id}/approve", h.approveMember)
	r.With(rbac.Require(rbac.PermissionOrgManageMembers)).Patch("/api/members/{id}/department", h.updateMemberDepartment)
	r.With(rbac.Require(rbac.PermissionOrgManageMembers)).Patch("/api/members/{id}/role", h.updateMemberRole)

	r.With(rbac.Require(rbac.PermissionOrgManageDepartments)).Get("/api/departments", h.listDepartments)
	r.With(rbac.Require(rbac.PermissionOrgManageDepartments)).Post("/api/departments", h.createDepartment)
	r.With(rbac.Require(rbac.PermissionOrgManageDepartments)).Patch("/api/departments/{id}/lead", h.setDepartmentLead)

	r.Get("/api/roles", h.listRoles)
	r.With(rbac.Require(rbac.PermissionOrgManageRoles)).Post("/api/roles", h.createRole)
	r.With(rbac.Require(rbac.PermissionOrgManageRoles)).Get("/api/roles/{id}/permissions", h.getRolePermissions)
	r.With(rbac.Require(rbac.PermissionOrgManageRoles)).Put("/api/roles/{id}/permissions", h.putRolePermissions)

	r.With(rbac.Require(rbac.PermissionOrgManageIdentitySources)).Get("/api/identity-configs/{provider}", h.getIdentityConfig)
	r.With(rbac.Require(rbac.PermissionOrgManageIdentitySources)).Put("/api/identity-configs/{provider}", h.putIdentityConfig)
	r.With(rbac.Require(rbac.PermissionOrgManageIdentitySources)).Get("/api/auth-settings", h.getAuthSettings)
	r.With(rbac.Require(rbac.PermissionOrgManageIdentitySources)).Put("/api/auth-settings", h.putAuthSettings)

	r.With(rbac.Require(rbac.PermissionOrgManageNotifyChannels)).Get("/api/notify-channels/{kind}", h.getNotifyChannel)
	r.With(rbac.Require(rbac.PermissionOrgManageNotifyChannels)).Put("/api/notify-channels/{kind}", h.putNotifyChannel)
	r.With(rbac.Require(rbac.PermissionOrgManageNotifyChannels)).Post("/api/notify-channels/{kind}/test", h.testNotifyChannel)
}
