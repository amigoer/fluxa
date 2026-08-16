// Feishu login: the OAuth redirect pair and the join-or-create rule that
// turns an external identity into a member.

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/user/identity"
)

// -- Feishu login -----------------------------------------------------------

func (h *Handler) feishuLogin(w http.ResponseWriter, r *http.Request) {
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

	member, err := h.findOrCreateFromExternalIdentity(r.Context(), types.IdentityProviderFeishu, info)
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

func (h *Handler) findOrCreateFromExternalIdentity(ctx context.Context, provider types.IdentityProvider, info identity.UserInfo) (types.Member, error) {
	member, err := h.repo.FindMemberByExternalIdentity(ctx, provider, info.ExternalUserID)
	if err == nil {
		return member, nil
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return types.Member{}, err
	}

	org, err := h.repo.GetOrganization(ctx)
	if err != nil {
		return types.Member{}, err
	}
	roles, err := h.service.EnsureBuiltinRoles(ctx, org.ID)
	if err != nil {
		return types.Member{}, err
	}

	email := info.Email
	member, err = h.repo.CreateMember(ctx, types.Member{
		OrgID:  org.ID,
		RoleID: roles[types.RoleEmployee].ID,
		Name:   info.Name,
		Email:  &email,
		Status: types.MemberStatusActive,
	})
	if err != nil {
		return types.Member{}, err
	}
	if err := h.repo.LinkExternalIdentity(ctx, member.ID, provider, info.ExternalUserID); err != nil {
		return types.Member{}, err
	}
	return member, nil
}
