package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

// authMethods reports which login paths are actually usable right now,
// so the login page can hide or grey out a method instead of sending
// the caller into a dead end (e.g. a full-page redirect to Feishu OAuth
// that isn't configured yet just 404s). Public: a caller who isn't
// logged in yet is exactly who needs to know this before picking a
// button.
func (h *Handler) authMethods(w http.ResponseWriter, r *http.Request) {
	feishu, err := h.service.GetIdentityConfig(r.Context(), types.IdentityProviderFeishu)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
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
	for _, kind := range []types.NotifyChannelKind{types.NotifyChannelSMS, types.NotifyChannelEmail} {
		channel, err := h.service.GetNotifyChannel(ctx, kind)
		if errors.Is(err, repo.ErrNotFound) {
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

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Logout(r.Context(), w, r)
	httpx.JSON(w, http.StatusOK, nil)
}
