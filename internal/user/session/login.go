package session

import (
	"context"
	"net/http"
	"time"
)

// Login creates a new session for memberID and sets it on the response
// as an HttpOnly cookie.
func (sm *Manager) Login(ctx context.Context, w http.ResponseWriter, memberID string) error {
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
func (sm *Manager) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
