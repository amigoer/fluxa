package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/user/identity"
)

const otpTTL = 5 * time.Minute

// errNoNotifyChannel marks the one OTP failure that is a deployment
// misconfiguration rather than a fault: local accounts are switched on
// but no SMS or email channel exists to carry the code. Callers turn it
// into a 503 the caller can act on instead of an opaque 500.
var errNoNotifyChannel = errors.New("user: no enabled notify channel configured")

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
}

func NewHandler(service *Service, repo *Repo, sessions *SessionManager, baseURL string) *Handler {
	return &Handler{
		service:  service,
		repo:     repo,
		sessions: sessions,
		feishu:   identity.NewFeishuAdapter(),
		baseURL:  baseURL,
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

// -- Feishu login -----------------------------------------------------------

func (h *Handler) feishuLogin(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.service.GetIdentityConfig(r.Context(), IdentityProviderFeishu)
	if errors.Is(err, ErrNotFound) || !cfg.Enabled {
		httpx.Error(w, http.StatusNotFound, i18n.KeyNotFound, "feishu login is not configured")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	redirectURI := h.baseURL + cfg.CallbackPath
	authorizeURL := "https://open.feishu.cn/open-apis/authen/v1/index?" + url.Values{
		"app_id":       {cfg.AppID},
		"redirect_uri": {redirectURI},
	}.Encode()

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *Handler) feishuCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, "missing code")
		return
	}

	cfg, err := h.service.GetIdentityConfig(r.Context(), IdentityProviderFeishu)
	if err != nil || !cfg.Enabled {
		httpx.Error(w, http.StatusNotFound, i18n.KeyNotFound, "feishu login is not configured")
		return
	}

	info, err := h.feishu.ExchangeCode(r.Context(), cfg.AppID, cfg.AppSecret, code)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, i18n.KeyInvalidCredentials, err.Error())
		return
	}

	member, err := h.findOrCreateFromExternalIdentity(r.Context(), IdentityProviderFeishu, info)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if member.Status == MemberStatusPendingReview {
		httpx.Error(w, http.StatusForbidden, i18n.KeyAccountPendingReview, "")
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) findOrCreateFromExternalIdentity(ctx context.Context, provider IdentityProvider, info identity.UserInfo) (Member, error) {
	member, err := h.repo.FindMemberByExternalIdentity(ctx, provider, info.ExternalUserID)
	if err == nil {
		return member, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Member{}, err
	}

	org, err := h.repo.GetOrganization(ctx)
	if err != nil {
		return Member{}, err
	}
	roles, err := h.service.EnsureBuiltinRoles(ctx, org.ID)
	if err != nil {
		return Member{}, err
	}

	email := info.Email
	member, err = h.repo.CreateMember(ctx, Member{
		OrgID:  org.ID,
		RoleID: roles[RoleEmployee].ID,
		Name:   info.Name,
		Email:  &email,
		Status: MemberStatusActive,
	})
	if err != nil {
		return Member{}, err
	}
	if err := h.repo.LinkExternalIdentity(ctx, member.ID, provider, info.ExternalUserID); err != nil {
		return Member{}, err
	}
	return member, nil
}

// -- Local account OTP login/registration ----------------------------------

type otpRequest struct {
	Identifier string `json:"identifier"`
}

func (h *Handler) sendOTP(ctx context.Context, identifier string, purpose OTPPurpose) error {
	code, err := identity.GenerateOTP()
	if err != nil {
		return err
	}
	if err := h.repo.CreateOTP(ctx, identifier, purpose, identity.HashOTP(code), time.Now().Add(otpTTL)); err != nil {
		return err
	}
	return h.deliverOTP(ctx, identifier, code)
}

// deliverOTP sends code through whichever channel (SMS or email) is
// configured for identifier's shape, using the pluggable notify package
// (DESIGN.md 7.1: not hardcoded to one vendor).
func (h *Handler) deliverOTP(ctx context.Context, identifier, code string) error {
	kind := NotifyChannelEmail
	if isPhone(identifier) {
		kind = NotifyChannelSMS
	}

	channel, err := h.service.GetNotifyChannel(ctx, kind)
	if errors.Is(err, ErrNotFound) || !channel.Enabled {
		return fmt.Errorf("%w: %s", errNoNotifyChannel, kind)
	}
	if err != nil {
		return err
	}
	// Same class of failure as no channel at all, and the admin needs the
	// same fix -- so say so, instead of letting the vendor call fail and
	// surface as a 500 to somebody trying to log in.
	if !notify.Configured(channel.Provider, channel.Config) {
		return fmt.Errorf("%w: %s is enabled but not configured", errNoNotifyChannel, kind)
	}

	message := fmt.Sprintf("Your Fluxa verification code is %s, valid for 5 minutes.", code)
	if kind == NotifyChannelSMS {
		err = notify.SendSMS(ctx, channel.Provider, channel.Config, identifier, code)
	} else {
		err = notify.SendEmail(ctx, channel.Provider, channel.Config, identifier, "Fluxa 验证码", message)
	}
	if err != nil {
		return err
	}
	return h.repo.LogNotifySent(ctx, kind, identifier, string(OTPPurposeLogin))
}

func isPhone(identifier string) bool {
	for _, r := range identifier {
		if r == '@' {
			return false
		}
	}
	return true
}

func (h *Handler) requestRegisterOTP(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetAuthSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !settings.LocalAccountEnabled {
		httpx.Error(w, http.StatusForbidden, i18n.KeyValidationFailed, "local accounts are disabled")
		return
	}

	var req otpRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.sendOTP(r.Context(), req.Identifier, OTPPurposeRegister); err != nil {
		writeOTPError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// writeOTPError separates "this deployment is misconfigured" from a
// genuine fault. A missing notify channel is the admin's to fix and the
// caller can be told so plainly; anything else is a 500.
func writeOTPError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoNotifyChannel) {
		httpx.Error(w, http.StatusServiceUnavailable, i18n.KeyNotifyChannelMissing, err.Error())
		return
	}
	httpx.InternalError(w, err)
}

type verifyRegisterRequest struct {
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
	Name       string `json:"name"`
}

func (h *Handler) verifyRegisterOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyRegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ok, err := h.repo.ConsumeOTP(r.Context(), req.Identifier, OTPPurposeRegister, identity.HashOTP(req.Code))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !ok {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyInvalidCredentials, "invalid or expired code")
		return
	}

	settings, err := h.service.GetAuthSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	org, err := h.repo.GetOrganization(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	roles, err := h.service.EnsureBuiltinRoles(r.Context(), org.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	status := MemberStatusActive
	if settings.LocalAccountRequiresApproval {
		status = MemberStatusPendingReview
	}

	member := Member{OrgID: org.ID, RoleID: roles[RoleEmployee].ID, Name: req.Name, Status: status}
	if isPhone(req.Identifier) {
		member.Phone = &req.Identifier
	} else {
		member.Email = &req.Identifier
	}
	member, err = h.repo.CreateMember(r.Context(), member)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	account := LocalAccount{MemberID: member.ID}
	if isPhone(req.Identifier) {
		account.Phone = &req.Identifier
	} else {
		account.Email = &req.Identifier
	}
	if _, err := h.repo.CreateLocalAccount(r.Context(), account); err != nil {
		httpx.InternalError(w, err)
		return
	}

	if status == MemberStatusPendingReview {
		httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "pending_review"})
		return
	}
	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, member)
}

func (h *Handler) requestLoginOTP(w http.ResponseWriter, r *http.Request) {
	var req otpRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if _, err := h.repo.FindLocalAccountByIdentifier(r.Context(), req.Identifier); err != nil {
		// Do not reveal whether the identifier is registered.
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	if err := h.sendOTP(r.Context(), req.Identifier, OTPPurposeLogin); err != nil {
		writeOTPError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type verifyLoginRequest struct {
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
}

func (h *Handler) verifyLoginOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ok, err := h.repo.ConsumeOTP(r.Context(), req.Identifier, OTPPurposeLogin, identity.HashOTP(req.Code))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !ok {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyInvalidCredentials, "invalid or expired code")
		return
	}

	account, err := h.repo.FindLocalAccountByIdentifier(r.Context(), req.Identifier)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyInvalidCredentials, "")
		return
	}
	member, err := h.repo.GetMember(r.Context(), account.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if member.Status == MemberStatusPendingReview {
		httpx.Error(w, http.StatusForbidden, i18n.KeyAccountPendingReview, "")
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, member)
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

// -- Members ----------------------------------------------------------------

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var departmentID *string
	if v := r.URL.Query().Get("departmentId"); v != "" {
		departmentID = &v
	}
	members, err := h.service.ListMembers(r.Context(), principal.OrgID, departmentID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, members)
}

func (h *Handler) approveMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.ApproveMember(r.Context(), id); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type updateDepartmentRequest struct {
	DepartmentID *string `json:"departmentId"`
}

func (h *Handler) updateMemberDepartment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateDepartmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.UpdateMemberDepartment(r.Context(), id, req.DepartmentID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type updateRoleRequest struct {
	RoleID string `json:"roleId"`
}

func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.UpdateMemberRole(r.Context(), id, req.RoleID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Departments --------------------------------------------------------

func (h *Handler) listDepartments(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	departments, err := h.service.ListDepartments(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, departments)
}

type createDepartmentRequest struct {
	Name string `json:"name"`
}

func (h *Handler) createDepartment(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var req createDepartmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	dept, err := h.service.CreateDepartment(r.Context(), principal.OrgID, req.Name)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dept)
}

type setLeadRequest struct {
	MemberID *string `json:"memberId"`
}

func (h *Handler) setDepartmentLead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setLeadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetDepartmentLead(r.Context(), id, req.MemberID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Roles --------------------------------------------------------------

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	roles, err := h.service.ListRoles(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, roles)
}

type createRoleRequest struct {
	Name        string            `json:"name"`
	Permissions []rbac.Permission `json:"permissions"`
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var req createRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	role, err := h.service.CreateCustomRole(r.Context(), principal.OrgID, req.Name, req.Permissions)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, role)
}

func (h *Handler) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	codes, err := h.service.RolePermissions(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, codes)
}

type setRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

func (h *Handler) putRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setRolePermissionsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetRolePermissions(r.Context(), id, req.Permissions); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Identity configs & auth settings -------------------------------------

func (h *Handler) getIdentityConfig(w http.ResponseWriter, r *http.Request) {
	provider := IdentityProvider(chi.URLParam(r, "provider"))
	cfg, err := h.service.GetIdentityConfig(r.Context(), provider)
	if errors.Is(err, ErrNotFound) {
		httpx.JSON(w, http.StatusOK, IdentityConfig{Provider: provider})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	cfg.AppSecret = maskSecret(cfg.AppSecret)
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) putIdentityConfig(w http.ResponseWriter, r *http.Request) {
	provider := IdentityProvider(chi.URLParam(r, "provider"))
	var cfg IdentityConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}
	cfg.Provider = provider
	if err := h.service.UpsertIdentityConfig(r.Context(), cfg); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handler) getAuthSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetAuthSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

func (h *Handler) putAuthSettings(w http.ResponseWriter, r *http.Request) {
	var settings AuthSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	if err := h.service.UpdateAuthSettings(r.Context(), settings); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Notify channels ------------------------------------------------------

// maskedValue is what a stored credential reads back as. It carries no
// prefix of the real value on purpose -- unlike an OAuth app id, an SMTP
// password has no half worth showing.
const maskedValue = "****"

// secretConfigKeys names the credential fields inside each channel kind's
// config blob. They are write-only: masked on the way out, and an update
// that leaves one blank (or hands back the mask) keeps what is stored.
var secretConfigKeys = map[NotifyChannelKind][]string{
	NotifyChannelSMS:   {"access_key_secret"},
	NotifyChannelEmail: {"password"},
}

func maskChannelSecrets(kind NotifyChannelKind, config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		out[k] = v
	}
	for _, k := range secretConfigKeys[kind] {
		if s, _ := out[k].(string); s != "" {
			out[k] = maskedValue
		}
	}
	return out
}

// mergeChannelSecrets puts the stored credential back wherever the caller
// did not supply a new one, so saving an unrelated field (or toggling the
// channel on) cannot silently blank the password.
func mergeChannelSecrets(kind NotifyChannelKind, stored, incoming map[string]any) map[string]any {
	if incoming == nil {
		incoming = map[string]any{}
	}
	for _, k := range secretConfigKeys[kind] {
		if s, _ := incoming[k].(string); s != "" && s != maskedValue {
			continue
		}
		if kept, ok := stored[k]; ok {
			incoming[k] = kept
		} else {
			delete(incoming, k)
		}
	}
	return incoming
}

func (h *Handler) getNotifyChannel(w http.ResponseWriter, r *http.Request) {
	kind := NotifyChannelKind(chi.URLParam(r, "kind"))
	channel, err := h.service.GetNotifyChannel(r.Context(), kind)
	if errors.Is(err, ErrNotFound) {
		// Same shape as the found case below (channel + sentThisMonth),
		// just zeroed out -- the frontend always expects the wrapper,
		// unconfigured or not.
		httpx.JSON(w, http.StatusOK, map[string]any{"channel": NotifyChannel{Kind: kind}, "sentThisMonth": 0})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	sentThisMonth, err := h.repo.CountNotifySentThisMonth(r.Context(), kind)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	channel.Config = maskChannelSecrets(kind, channel.Config)
	httpx.JSON(w, http.StatusOK, map[string]any{"channel": channel, "sentThisMonth": sentThisMonth})
}

func (h *Handler) putNotifyChannel(w http.ResponseWriter, r *http.Request) {
	kind := NotifyChannelKind(chi.URLParam(r, "kind"))
	var channel NotifyChannel
	if !decodeJSON(w, r, &channel) {
		return
	}
	channel.Kind = kind

	stored, err := h.service.GetNotifyChannel(r.Context(), kind)
	if err != nil && !errors.Is(err, ErrNotFound) {
		httpx.InternalError(w, err)
		return
	}
	channel.Config = mergeChannelSecrets(kind, stored.Config, channel.Config)

	// Refuse to switch on a channel that cannot send. Every "is a channel
	// enabled?" check downstream -- including the one that decides whether
	// the login page offers local accounts -- treats the flag as a promise
	// that a code will arrive.
	if channel.Enabled && !notify.Configured(channel.Provider, channel.Config) {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed,
			"the channel is missing required credentials and cannot be enabled")
		return
	}

	if err := h.service.UpsertNotifyChannel(r.Context(), channel); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type testChannelRequest struct {
	Recipient string `json:"recipient"`
}

// testNotifyChannel sends one real message through the stored config, so
// an admin learns whether the credentials work from this button rather
// than from a colleague whose verification code never arrived.
//
// It deliberately uses what is saved rather than what is on screen: that
// is the config the OTP path will use, and since secrets are write-only
// the form does not hold them anyway. It also ignores `enabled` -- the
// whole point is to check a channel before switching it on.
func (h *Handler) testNotifyChannel(w http.ResponseWriter, r *http.Request) {
	kind := NotifyChannelKind(chi.URLParam(r, "kind"))
	if kind != NotifyChannelEmail {
		httpx.Error(w, http.StatusNotImplemented, i18n.KeyValidationFailed, "only the email channel can be tested for now")
		return
	}

	var req testChannelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Recipient == "" {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, "recipient is required")
		return
	}

	channel, err := h.service.GetNotifyChannel(r.Context(), kind)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyNotifyChannelMissing, "the email channel has no configuration yet")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	body := "这是一封来自 Fluxa 的测试邮件。\r\n\r\n" +
		"收到它说明发信通道配置正确，本地账号的注册和登录验证码可以正常送达。"
	if err := notify.SendEmail(r.Context(), channel.Provider, channel.Config, req.Recipient, "Fluxa 测试邮件", body); err != nil {
		// The upstream message is the entire value of this button:
		// "authentication failed" and "connection timed out" need
		// different fixes, and only the relay knows which happened.
		httpx.Error(w, http.StatusBadGateway, i18n.KeyValidationFailed, err.Error())
		return
	}
	if err := h.repo.LogNotifySent(r.Context(), kind, req.Recipient, "test"); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
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
