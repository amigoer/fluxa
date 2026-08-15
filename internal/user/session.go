package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/rbac"
)

// sessionTTL is how long a session stays valid without the member
// logging in again. There is no sliding renewal in v1: it is simpler,
// and a week is generous enough for daily internal-tool use.
const sessionTTL = 7 * 24 * time.Hour

// SessionManager implements the server-side session described in
// DESIGN.md 7.1: the client only ever holds an opaque random token in a
// cookie, and the server stores a hash of it in Postgres. Revoking a
// session (an admin forcing someone out, or a plain logout) takes effect
// on the very next request, which is the whole reason this was chosen
// over a stateless JWT.
type SessionManager struct {
	repo         *Repo
	users        *Service
	cookieName   string
	cookieSecure bool
}

func NewSessionManager(repo *Repo, users *Service, cookieName string, cookieSecure bool) *SessionManager {
	return &SessionManager{repo: repo, users: users, cookieName: cookieName, cookieSecure: cookieSecure}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Login creates a new session for memberID and sets it on the response
// as an HttpOnly cookie.
func (sm *SessionManager) Login(ctx context.Context, w http.ResponseWriter, memberID string) error {
	token, err := generateToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(sessionTTL)
	if err := sm.repo.CreateSession(ctx, hashToken(token), memberID, expiresAt); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   sm.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Logout revokes the session tied to the request's cookie, if any, and
// clears the cookie.
func (sm *SessionManager) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sm.cookieName); err == nil {
		_ = sm.repo.RevokeSession(ctx, hashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   sm.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Middleware authenticates the request from its session cookie, loads
// the member's rbac.Principal, and stores it on the context for
// rbac.Require (and handlers) to read. Requests without a valid session
// are rejected here so downstream handlers never have to check again.
func (sm *SessionManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sm.cookieName)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
			return
		}

		session, err := sm.repo.GetSession(r.Context(), hashToken(cookie.Value))
		if errors.Is(err, ErrNotFound) {
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
