package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/user/identity"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

const otpTTL = 5 * time.Minute

// errNoNotifyChannel marks the one OTP failure that is a deployment
// misconfiguration rather than a fault: local accounts are switched on
// but no SMS or email channel exists to carry the code. Callers turn it
// into a 503 the caller can act on instead of an opaque 500.
var errNoNotifyChannel = errors.New("user: no enabled notify channel configured")

func (h *Handler) sendOTP(ctx context.Context, identifier string, purpose types.OTPPurpose) error {
	if err := h.checkOTPQuota(ctx, identifier); err != nil {
		return err
	}

	code, err := identity.GenerateOTP()
	if err != nil {
		return err
	}
	if err := h.repo.CreateOTP(ctx, identifier, purpose, identity.HashOTP(code), time.Now().Add(otpTTL)); err != nil {
		return err
	}
	return h.deliverOTP(ctx, identifier, code, purpose)
}

// deliverOTP sends code through whichever channel (SMS or email) is
// configured for identifier's shape, using the pluggable notify package
// (DESIGN.md 7.1: not hardcoded to one vendor).
func (h *Handler) deliverOTP(ctx context.Context, identifier, code string, purpose types.OTPPurpose) error {
	kind := types.NotifyChannelEmail
	if isPhone(identifier) {
		kind = types.NotifyChannelSMS
	}

	channel, err := h.service.GetNotifyChannel(ctx, kind)
	if errors.Is(err, repo.ErrNotFound) || !channel.Enabled {
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

	// SMS carries the bare code and nothing else: the wording lives in the
	// template the vendor registered and approved, and anything we sent
	// alongside it would be discarded. Mail is ours end to end, so it goes
	// as the full branded message.
	if kind == types.NotifyChannelSMS {
		err = notify.SendSMS(ctx, channel.Provider, channel.Config, identifier, code)
	} else {
		err = notify.SendEmail(ctx, channel.Provider, channel.Config, identifier, notify.OTPMail(code, otpTTL, h.service.MailBrand(ctx)))
	}
	if err != nil {
		// Recorded before returning: "我没收到验证码" is otherwise
		// unanswerable from the server side, since the request itself
		// leaves no trace of having tried.
		_ = h.repo.LogNotifyFailed(ctx, kind, identifier, string(purpose), err)
		return err
	}
	return h.repo.LogNotifySent(ctx, kind, identifier, string(purpose))
}

func isPhone(identifier string) bool {
	for _, r := range identifier {
		if r == '@' {
			return false
		}
	}
	return true
}
