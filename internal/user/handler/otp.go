package handler

import (
	"errors"
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/user/identity"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

type otpRequest struct {
	Identifier string `json:"identifier"`
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

	// Somebody already here who signed up again -- an employee created by
	// the IM directory before it was switched off, most likely. Proving
	// control of the address is the same proof signing in would need, so
	// they get their existing member back rather than a second one with
	// none of their history on it.
	if existing, err := h.repo.FindMemberByIdentifier(r.Context(), req.Identifier); err == nil {
		if err := h.ensureLocalAccount(r.Context(), existing, req.Identifier); err != nil {
			httpx.InternalError(w, err)
			return
		}
		if existing.Status == types.MemberStatusPendingReview {
			httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "pending_review"})
			return
		}
		if err := h.sessions.Login(r.Context(), w, existing.ID); err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, existing)
		return
	} else if !errors.Is(err, repo.ErrNotFound) {
		httpx.InternalError(w, err)
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
	// Gated like registration is: an admin who has switched local
	// accounts off has switched off this way in, and the codes stop
	// going out. Without this check the toggle only hid the sign-up
	// form and anybody with an existing account kept their way in.
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

	// Ahead of the account lookup on purpose: the burst windows count
	// registered and unknown addresses alike, so a 429 here says nothing
	// about whether the address exists. The durable daily cap inside
	// sendOTP can only accumulate for a real account, so it stays behind
	// this uniform gate rather than being the first thing a prober meets.
	if !h.allowOTPRequest(r, req.Identifier) {
		writeOTPError(w, errOTPRateLimited)
		return
	}

	// A local account row is one way to be known here, not the only one:
	// a member created by an identity source has none, and would be
	// locked out the day that source is switched off -- or would sign up
	// again and end up as a second copy of themselves. Falling back to
	// the member's own address covers the switch without either.
	if !h.knownForLocalLogin(r.Context(), req.Identifier) {
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

	member, err := h.memberForLocalLogin(r.Context(), req.Identifier)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			httpx.Error(w, http.StatusBadRequest, i18n.KeyInvalidCredentials, "")
			return
		}
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
