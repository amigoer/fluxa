package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

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

// errOTPRateLimited is returned once any of the four limits is hit. It is
// deliberately one error rather than four: telling the caller which limit
// they tripped tells an attacker how to pace the next attempt.
var errOTPRateLimited = errors.New("user: too many verification code requests")

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
