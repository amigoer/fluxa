package session

import (
	"errors"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/user/repo"
)

// CurrentMember reports who the request is already signed in as, for the
// public routes that are not behind Middleware but still care. It answers
// false for any reason a session is unusable rather than distinguishing
// them: the callers here choose a path, they do not report an error.
func (sm *Manager) CurrentMember(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return "", false
	}
	session, err := sm.repo.GetSession(r.Context(), hashToken(cookie.Value))
	if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return "", false
	}
	return session.MemberID, true
}

// Middleware authenticates the request from its session cookie, loads
// the member's rbac.Principal, and stores it on the context for
// rbac.Require (and handlers) to read. Requests without a valid session
// are rejected here so downstream handlers never have to check again.
func (sm *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sm.cookieName)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
			return
		}

		session, err := sm.repo.GetSession(r.Context(), hashToken(cookie.Value))
		if errors.Is(err, repo.ErrNotFound) {
			httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
			return
		}
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
			httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
			return
		}

		principal, err := sm.users.LoadPrincipal(r.Context(), session.MemberID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(rbac.WithPrincipal(r.Context(), principal)))
	})
}
