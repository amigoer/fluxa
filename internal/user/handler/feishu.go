// Feishu login: the OAuth redirect pair and the join-or-create rule that
// turns an external identity into a member.

package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/user/identity"
)

// -- Feishu login -----------------------------------------------------------

// bindState marks an authorization the caller started from the "link my
// account" entry rather than the login button. Binding keys off this and
// not merely off a session being present: on a shared browser the
// session belongs to whoever used it last, and silently attaching a
// second person's Feishu account to it is worse than the duplicate
// member that binding exists to prevent.
const bindState = "bind"

func (h *Handler) feishuLogin(w http.ResponseWriter, r *http.Request) {
	h.feishuAuthorize(w, r, "")
}

// feishuBind starts the same authorization from inside a session, so the
// identity it returns is attached to the member who asked for it.
func (h *Handler) feishuBind(w http.ResponseWriter, r *http.Request) {
	h.feishuAuthorize(w, r, bindState)
}

func (h *Handler) feishuAuthorize(w http.ResponseWriter, r *http.Request, state string) {
	cfg, err := h.service.GetIdentityConfig(r.Context(), types.IdentityProviderFeishu)
	if errors.Is(err, repo.ErrNotFound) || !cfg.Enabled {
		httpx.Error(w, http.StatusNotFound, i18n.KeyNotFound, "feishu login is not configured")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	redirectURI := h.baseURL + cfg.CallbackPath
	params := url.Values{
		"app_id":       {cfg.AppID},
		"redirect_uri": {redirectURI},
	}
	if state != "" {
		params.Set("state", state)
	}
	authorizeURL := "https://open.feishu.cn/open-apis/authen/v1/index?" + params.Encode()

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *Handler) feishuCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, "missing code")
		return
	}

	cfg, err := h.service.GetIdentityConfig(r.Context(), types.IdentityProviderFeishu)
	if err != nil || !cfg.Enabled {
		httpx.Error(w, http.StatusNotFound, i18n.KeyNotFound, "feishu login is not configured")
		return
	}

	info, err := h.feishu.ExchangeCode(r.Context(), cfg.AppID, cfg.AppSecret, code)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, i18n.KeyInvalidCredentials, err.Error())
		return
	}
	// Feishu returns the profile fields the app is permitted to see and
	// silently omits the rest, so a member landing here with no address
	// looks like our bug rather than a permission that was never granted.
	// Say which it is, once per sign-in, where an admin will find it.
	if info.Email == "" {
		slog.WarnContext(r.Context(), "user: feishu returned no email for this account; grant the app 获取用户邮箱信息 in the Feishu developer console if members should carry one",
			"external_user_id", info.ExternalUserID)
	}

	member, err := h.resolveExternalIdentity(r, types.IdentityProviderFeishu, info)
	if errors.Is(err, errBindWithoutSession) {
		httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if member.Status == types.MemberStatusPendingReview {
		httpx.Error(w, http.StatusForbidden, i18n.KeyAccountPendingReview, "")
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// findOrCreateFromExternalIdentity resolves a sign-in at an identity
// source to a member of this deployment. An external identity is a way
// to prove who you are, not an account of its own, so this walks three
// steps in order and only reaches the last one when the person really is
// new here:
//
//  1. The identity is already bound to a member -- sign that member in.
//  2. The caller started this from the bind entry and is signed in --
//     attach the identity to the member they are signed in as. This is
//     the deliberate "link my Feishu account" path, and the only one
//     that needs nothing from the source beyond the identity itself, so
//     it works whatever profile permissions the app was granted.
//  3. The source vouches for an address a member already has -- bind the
//     identity to that member. This is the case a deployment runs into
//     the first time it turns an identity source on: everybody already
//     has an account from before it was configured, and without this
//     step every one of them would be issued a second one and see two of
//     themselves in the member list.
//  4. Nobody here matches -- create a member, as before.
func (h *Handler) resolveExternalIdentity(r *http.Request, provider types.IdentityProvider, info identity.UserInfo) (types.Member, error) {
	ctx := r.Context()
	member, err := h.repo.FindMemberByExternalIdentity(ctx, provider, info.ExternalUserID)
	if err == nil {
		// The source owns the display name, work address and photo, so a
		// sign-in is the moment to take the current ones rather than keep
		// whatever they were on the day this member first appeared.
		return h.refreshFromSource(ctx, member.ID, info)
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return types.Member{}, err
	}

	if r.URL.Query().Get("state") == bindState {
		memberID, ok := h.sessions.CurrentMember(r)
		if !ok {
			return types.Member{}, errBindWithoutSession
		}
		if err := h.repo.LinkExternalIdentity(ctx, memberID, provider, info.ExternalUserID); err != nil {
			return types.Member{}, err
		}
		slog.InfoContext(ctx, "user: bound external identity to the signed-in member",
			"provider", provider, "member_id", memberID)
		return h.refreshFromSource(ctx, memberID, info)
	}

	if info.Email != "" {
		existing, err := h.repo.FindMemberByIdentifier(ctx, info.Email)
		if err == nil {
			if err := h.repo.LinkExternalIdentity(ctx, existing.ID, provider, info.ExternalUserID); err != nil {
				return types.Member{}, err
			}
			slog.InfoContext(ctx, "user: bound external identity to the existing member with this address",
				"provider", provider, "member_id", existing.ID)
			return h.refreshFromSource(ctx, existing.ID, info)
		}
		if !errors.Is(err, repo.ErrNotFound) {
			return types.Member{}, err
		}
	}

	org, err := h.repo.GetOrganization(ctx)
	if err != nil {
		return types.Member{}, err
	}
	roles, err := h.service.EnsureBuiltinRoles(ctx, org.ID)
	if err != nil {
		return types.Member{}, err
	}

	member, err = h.repo.CreateMember(ctx, types.Member{
		OrgID:     org.ID,
		RoleID:    roles[types.RoleEmployee].ID,
		Name:      info.Name,
		Email:     optional(info.Email),
		AvatarURL: optional(info.AvatarURL),
		Status:    types.MemberStatusActive,
	})
	if err != nil {
		return types.Member{}, err
	}
	if err := h.repo.LinkExternalIdentity(ctx, member.ID, provider, info.ExternalUserID); err != nil {
		return types.Member{}, err
	}
	return member, nil
}

// refreshFromSource takes the current profile from the identity source
// onto an existing member and returns it as stored.
func (h *Handler) refreshFromSource(ctx context.Context, memberID string, info identity.UserInfo) (types.Member, error) {
	if err := h.repo.UpdateMemberProfile(ctx, memberID, info.Name, optional(info.Email), optional(info.AvatarURL)); err != nil {
		return types.Member{}, err
	}
	return h.repo.GetMember(ctx, memberID)
}

// errBindWithoutSession is the bind flow arriving with no session to
// bind to -- the session expired while the caller was at Feishu. Falling
// through to "create a member" would hand them a second account, which
// is the outcome the bind entry exists to avoid, so it stops instead.
var errBindWithoutSession = errors.New("user: bind requires an active session")

// optional turns a field the provider may simply not have returned into
// a NULL rather than an empty string. Storing "" made the members table
// show a colleague with a blank address where "no address on file" was
// the truth, and made every "has an email?" check a comparison against
// the empty string instead of a null check.
func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
