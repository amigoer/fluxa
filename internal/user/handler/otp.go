// Local account registration and login by one-time code, and the limits
// that keep those endpoints from being used to send mail and billable SMS
// to arbitrary addresses.

package handler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/user/identity"
)

const otpTTL = 5 * time.Minute

// OTP rate limits.
//
// Everything a caller can observe is enforced in memory, keyed on the
// address as typed, because those checks run before the account lookup
// and so answer identically for a registered and an unknown address. The
// one database-backed limit is the daily ceiling: it survives restarts
// and is shared across replicas, but it can only ever accumulate for a
// real account, which is why the login path never lets it change the
// reply (see requestLoginOTP).
const (
	otpCooldownWindow     = 60 * time.Second
	otpBurstPerIdentifier = 5
	otpBurstWindow        = 10 * time.Minute
	otpBurstPerIP         = 20
	otpIPWindow           = time.Hour
	otpDailyPerIdentifier = 10
)

// errNoNotifyChannel marks the one OTP failure that is a deployment
// misconfiguration rather than a fault: local accounts are switched on
// but no SMS or email channel exists to carry the code. Callers turn it
// into a 503 the caller can act on instead of an opaque 500.
var errNoNotifyChannel = errors.New("user: no enabled notify channel configured")

// errOTPRateLimited is returned once any of the four limits is hit. It is
// deliberately one error rather than four: telling the caller which limit
// they tripped tells an attacker how to pace the next attempt.
var errOTPRateLimited = errors.New("user: too many verification code requests")

// -- Local account OTP login/registration ----------------------------------

type otpRequest struct {
	Identifier string `json:"identifier"`
}

// allowOTPRequest applies the burst limits that must answer identically
// whether or not the identifier belongs to an account.
//
// Order matters on the login path: these run before the account lookup,
// so a limited caller and an unknown address get the same reply. Checking
// after the lookup would have turned the limiter itself into an oracle
// for which addresses are registered.
func (h *Handler) allowOTPRequest(r *http.Request, identifier string) bool {
	// All three are consulted, not short-circuited: a caller who trips one
	// window should still have the attempt counted against the others,
	// otherwise the cheapest limit to trip would shield the rest.
	okCooldown := h.otpCooldown.Allow(identifier)
	okBurst := h.otpPerIdentifier.Allow(identifier)
	okIP := h.otpPerIP.Allow(clientIP(r))
	return okCooldown && okBurst && okIP
}

func clientIP(r *http.Request) string {
	// chi's RealIP middleware has already resolved this from the proxy
	// headers where one is in front; RemoteAddr is what is left otherwise.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// checkOTPQuota enforces the daily ceiling on codes sent to one address.
// It reads from local_otp_codes rather than memory so that it holds
// across a restart and across replicas -- the property that matters for
// the limit whose job is to bound how much mail one address can be made
// to receive, rather than to blunt a burst.
func (h *Handler) checkOTPQuota(ctx context.Context, identifier string) error {
	count, _, err := h.repo.CountOTPsSince(ctx, identifier, time.Now().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	if count >= otpDailyPerIdentifier {
		return errOTPRateLimited
	}
	return nil
}

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

	// SMS carries the bare code because the vendor's template supplies the
	// wording; email carries the whole sentence, in the console's language.
	body := fmt.Sprintf("你的 Fluxa 验证码是 %s，5 分钟内有效。\r\n\r\n如果这不是你本人的操作，忽略这封邮件即可。", code)
	if kind == types.NotifyChannelSMS {
		err = notify.SendSMS(ctx, channel.Provider, channel.Config, identifier, code)
	} else {
		err = notify.SendEmail(ctx, channel.Provider, channel.Config, identifier, "Fluxa 验证码", body)
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
	if !h.allowOTPRequest(r, req.Identifier) {
		writeOTPError(w, errOTPRateLimited)
		return
	}
	if err := h.sendOTP(r.Context(), req.Identifier, types.OTPPurposeRegister); err != nil {
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
	if errors.Is(err, errOTPRateLimited) {
		httpx.Error(w, http.StatusTooManyRequests, i18n.KeyTooManyRequests, err.Error())
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

	ok, err := h.repo.ConsumeOTP(r.Context(), req.Identifier, types.OTPPurposeRegister, identity.HashOTP(req.Code))
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

	status := types.MemberStatusActive
	if settings.LocalAccountRequiresApproval {
		status = types.MemberStatusPendingReview
	}

	member := types.Member{OrgID: org.ID, RoleID: roles[types.RoleEmployee].ID, Name: req.Name, Status: status}
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

	account := types.LocalAccount{MemberID: member.ID}
	if isPhone(req.Identifier) {
		account.Phone = &req.Identifier
	} else {
		account.Email = &req.Identifier
	}
	if _, err := h.repo.CreateLocalAccount(r.Context(), account); err != nil {
		httpx.InternalError(w, err)
		return
	}

	if status == types.MemberStatusPendingReview {
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

	// Ahead of the account lookup on purpose: the burst windows count
	// registered and unknown addresses alike, so a 429 here says nothing
	// about whether the address exists. The durable daily cap inside
	// sendOTP can only accumulate for a real account, so it stays behind
	// this uniform gate rather than being the first thing a prober meets.
	if !h.allowOTPRequest(r, req.Identifier) {
		writeOTPError(w, errOTPRateLimited)
		return
	}

	if _, err := h.repo.FindLocalAccountByIdentifier(r.Context(), req.Identifier); err != nil {
		// Do not reveal whether the identifier is registered.
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	if err := h.sendOTP(r.Context(), req.Identifier, types.OTPPurposeLogin); err != nil {
		if errors.Is(err, errOTPRateLimited) {
			// The daily ceiling only ever accumulates for an address that
			// has an account, so answering 429 here would confirm one
			// exists. Give the same reply an unknown address gets; the
			// windows above are what tell an honest caller to slow down.
			httpx.JSON(w, http.StatusOK, nil)
			return
		}
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

	ok, err := h.repo.ConsumeOTP(r.Context(), req.Identifier, types.OTPPurposeLogin, identity.HashOTP(req.Code))
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
	if member.Status == types.MemberStatusPendingReview {
		httpx.Error(w, http.StatusForbidden, i18n.KeyAccountPendingReview, "")
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, member)
}
