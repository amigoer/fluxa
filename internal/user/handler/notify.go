// Notify channel configuration: the SMS and email credentials the OTP
// codes go out on, including the write-only handling that keeps stored
// secrets from being read back.

package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"

	// Aliased to keep it distinct from the project's own notify/mail
	// package, which this file also reaches (through notify.SendEmail).
	netmail "net/mail"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
)

// -- Notify channels ------------------------------------------------------

// maskedValue is what a stored credential reads back as. It carries no
// prefix of the real value on purpose -- unlike an OAuth app id, an SMTP
// password has no half worth showing.
const maskedValue = "****"

// secretConfigKeys names the credential fields inside each channel kind's
// config blob. They are write-only: masked on the way out, and an update
// that leaves one blank (or hands back the mask) keeps what is stored.
var secretConfigKeys = map[types.NotifyChannelKind][]string{
	types.NotifyChannelSMS:   {"access_key_secret"},
	types.NotifyChannelEmail: {"password"},
}

func maskChannelSecrets(kind types.NotifyChannelKind, config map[string]any) map[string]any {
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
func mergeChannelSecrets(kind types.NotifyChannelKind, stored, incoming map[string]any) map[string]any {
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
	kind := types.NotifyChannelKind(chi.URLParam(r, "kind"))
	channel, err := h.service.GetNotifyChannel(r.Context(), kind)
	if errors.Is(err, repo.ErrNotFound) {
		// Same shape as the found case below (channel + sentThisMonth),
		// just zeroed out -- the frontend always expects the wrapper,
		// unconfigured or not.
		httpx.JSON(w, http.StatusOK, map[string]any{"channel": types.NotifyChannel{Kind: kind}, "sentThisMonth": 0})
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
	kind := types.NotifyChannelKind(chi.URLParam(r, "kind"))
	var channel types.NotifyChannel
	if !decodeJSON(w, r, &channel) {
		return
	}
	channel.Kind = kind

	stored, err := h.service.GetNotifyChannel(r.Context(), kind)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		httpx.InternalError(w, err)
		return
	}
	channel.Config = mergeChannelSecrets(kind, stored.Config, channel.Config)

	// Refuse to switch on a channel that cannot send. Every "is a channel
	// enabled?" check downstream -- including the one that decides whether
	// the login page offers local accounts -- treats the flag as a promise
	// that a code will arrive.
	if channel.Enabled && !notify.Configured(channel.Provider, channel.Config) {
		// Its own key rather than the generic validation one: "请求参数有误"
		// tells an admin nothing about which box to go back and fill.
		httpx.Error(w, http.StatusBadRequest, i18n.KeyNotifyChannelIncomplete,
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
	kind := types.NotifyChannelKind(chi.URLParam(r, "kind"))
	if kind != types.NotifyChannelEmail {
		httpx.Error(w, http.StatusNotImplemented, i18n.KeyValidationFailed, "only the email channel can be tested for now")
		return
	}

	var req testChannelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// Parsed, not just checked for emptiness: the browser trims and type
	// checks the field, and anything calling the API directly does not.
	// A malformed address otherwise reaches the relay and comes back as a
	// confusing upstream error.
	recipient, err := parseEmailAddress(req.Recipient)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}

	channel, err := h.service.GetNotifyChannel(r.Context(), kind)
	if errors.Is(err, repo.ErrNotFound) {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyNotifyChannelMissing, "the email channel has no configuration yet")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	body := "这是一封来自 Fluxa 的测试邮件。\r\n\r\n" +
		"收到它说明发信通道配置正确，本地账号的注册和登录验证码可以正常送达。"
	if err := notify.SendEmail(r.Context(), channel.Provider, channel.Config, recipient, "Fluxa 测试邮件", body); err != nil {
		_ = h.repo.LogNotifyFailed(r.Context(), kind, recipient, notifyPurposeTest, err)
		// The upstream message is the entire value of this button:
		// "authentication failed" and "connection timed out" need
		// different fixes, and only the relay knows which happened. It
		// travels under its own key so the frontend renders the detail
		// rather than replacing it with generic validation copy.
		httpx.Error(w, http.StatusBadGateway, i18n.KeyNotifySendFailed, err.Error())
		return
	}
	if err := h.repo.LogNotifySent(r.Context(), kind, recipient, notifyPurposeTest); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// notifyPurposeTest marks a message sent by the 测试 button rather than by
// an authentication flow.
const notifyPurposeTest = "test"

// parseEmailAddress accepts what a mail server will: one addr-spec, with
// no display name and no group syntax.
func parseEmailAddress(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("recipient is required")
	}
	addr, err := netmail.ParseAddress(trimmed)
	if err != nil {
		return "", fmt.Errorf("%q is not a valid email address", trimmed)
	}
	if addr.Name != "" || addr.Address != trimmed {
		return "", fmt.Errorf("%q must be a bare email address", trimmed)
	}
	return addr.Address, nil
}
