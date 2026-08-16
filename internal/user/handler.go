package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/platform/ratelimit"
	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/user/identity"
)

// Handler wires the User module's HTTP surface: setup, login (Feishu +
// local OTP), session-authenticated member/department/role/identity
// config management. Admin and employee views share these same
// endpoints; rbac.Require on each route is what limits what a caller can
// see or do (DESIGN.md 6.3: "不拆成两个独立的 App").
type Handler struct {
	service  *Service
	repo     *Repo
	sessions *SessionManager
	feishu   *identity.FeishuAdapter
	baseURL  string

	// Limiters for the OTP endpoints, which are public, take an arbitrary
	// address, and now that the mail channel actually delivers, make the
	// server send real mail (and real, billable SMS) on demand.
	otpCooldown      *ratelimit.Window
	otpPerIdentifier *ratelimit.Window
	otpPerIP         *ratelimit.Window
}

func NewHandler(service *Service, repo *Repo, sessions *SessionManager, baseURL string) *Handler {
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

// -- Setup ----------------------------------------------------------------

func (h *Handler) setupStatus(w http.ResponseWriter, r *http.Request) {
	_, err := h.repo.GetOrganization(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]bool{"needsSetup": errors.Is(err, ErrNotFound)})
}

// authMethods reports which login paths are actually usable right now,
// so the login page can hide or grey out a method instead of sending
// the caller into a dead end (e.g. a full-page redirect to Feishu OAuth
// that isn't configured yet just 404s). Public: a caller who isn't
// logged in yet is exactly who needs to know this before picking a
// button.
func (h *Handler) authMethods(w http.ResponseWriter, r *http.Request) {
	feishu, err := h.service.GetIdentityConfig(r.Context(), IdentityProviderFeishu)
	if err != nil && !errors.Is(err, ErrNotFound) {
		httpx.InternalError(w, err)
		return
	}

	settings, err := h.service.GetAuthSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	// Being switched on is not the same as being usable: local accounts
	// authenticate purely by OTP, so with no notify channel to carry the
	// code there is nothing behind the button. Reporting the toggle alone
	// re-created exactly the dead end this endpoint exists to prevent --
	// the login page offered 手机号 / 邮箱登录 and 获取验证码 then failed.
	local := settings.LocalAccountEnabled
	if local {
		deliverable, err := h.canDeliverOTP(r.Context())
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		local = deliverable
	}

	httpx.JSON(w, http.StatusOK, map[string]bool{
		"feishu": feishu.Enabled,
		"local":  local,
	})
}

// canDeliverOTP reports whether any channel is configured to carry a
// one-time code right now. Either kind will do: the login form lets the
// caller identify themselves by phone or by email.
func (h *Handler) canDeliverOTP(ctx context.Context) (bool, error) {
	for _, kind := range []NotifyChannelKind{NotifyChannelSMS, NotifyChannelEmail} {
		channel, err := h.service.GetNotifyChannel(ctx, kind)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return false, err
		}
		// Enabled is only a promise; rows written before the guard in
		// putNotifyChannel existed can still be switched on with an empty
		// config, so the credentials get checked here rather than trusted.
		if channel.Enabled && notify.Configured(channel.Provider, channel.Config) {
			return true, nil
		}
	}
	return false, nil
}

type setupRequest struct {
	OrgName    string `json:"orgName"`
	AdminName  string `json:"adminName"`
	AdminEmail string `json:"adminEmail"`
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	if _, err := h.repo.GetOrganization(r.Context()); !errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusConflict, i18n.KeyValidationFailed, "organization already set up")
		return
	}

	var req setupRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	member, err := h.service.Bootstrap(r.Context(), req.OrgName, req.AdminName, req.AdminEmail)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, member)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Logout(r.Context(), w, r)
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	member, err := h.repo.GetMember(r.Context(), principal.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	role, err := h.repo.GetRoleByID(r.Context(), member.RoleID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	departmentName := ""
	if member.DepartmentID != nil {
		if dept, err := h.repo.GetDepartment(r.Context(), *member.DepartmentID); err == nil {
			departmentName = dept.Name
		}
	}

	// The org name is display-only, but every authenticated screen shows
	// it (the sidebar brand block), so it rides along with /api/me rather
	// than costing a second round trip on every page load.
	orgName := ""
	if org, err := h.repo.GetOrganization(r.Context()); err == nil {
		orgName = org.Name
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"member":         member,
		"permissions":    principal.Permissions,
		"roleName":       role.Name,
		"departmentName": departmentName,
		"orgName":        orgName,
	})
}

// -- helpers --------------------------------------------------------------

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return false
	}
	return true
}

func maskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:4] + "****"
}
