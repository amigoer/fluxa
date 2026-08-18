package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/user/types"
)

func (h *Handler) getMailSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetMailSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

func (h *Handler) putMailSettings(w http.ResponseWriter, r *http.Request) {
	var settings types.MailSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	if err := h.service.UpdateMailSettings(r.Context(), settings); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// previewCode is obviously not a real one. A preview showing six
// plausible digits is a preview somebody eventually pastes into a login
// form, so it reads as a sample at a glance.
const previewCode = "000000"

// previewMailSettings renders the verification mail exactly as it would
// be sent, from values that have not been saved yet -- so what an admin
// judges before saving is the real template and not a mock-up of it.
//
// It answers with the mail's own HTML rather than JSON: the caller drops
// it into a sandboxed frame, which is the only way to see it as a mail
// client would. That is also why the document must not be rendered into
// the console's own page, and why the frame that shows it runs with no
// privileges of its own.
func (h *Handler) previewMailTemplate(w http.ResponseWriter, r *http.Request) {
	var settings types.MailSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	mail := notify.OTPMail(previewCode, otpTTL, notify.Brand{
		OrgName: settings.OrgName,
		SignOff: settings.SignOff,
		Contact: settings.ContactLine,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Nothing here is meant to run or to be reachable from anywhere else.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(mail.HTML))
}
