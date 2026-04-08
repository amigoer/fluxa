// admin_auth.go — username/password login flow for the dashboard.
//
// Routes installed by this file:
//
//	POST /admin/auth/login    — exchange credentials for an opaque token
//	POST /admin/auth/logout   — revoke the caller's token
//	GET  /admin/auth/me       — return the currently authenticated user
//	POST /admin/auth/password — rotate the caller's own password
//
// The login endpoint is the only one in the admin surface that does
// NOT require a Bearer token — everything else flows through
// requireAuth, which now resolves the token to an admin_users row via
// the store.

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/amigoer/fluxa/internal/store"
)

// loginRequest is the JSON shape posted to /admin/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse returns the freshly minted token + user info so the
// dashboard can stash both in one round trip.
type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	User      userDTO `json:"user"`
}

// userDTO is the safe-to-serialise view of an admin account. The
// password hash is intentionally absent.
type userDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	CreatedAt string `json:"created_at,omitempty"`
}

func toUserDTO(u store.AdminUser) userDTO {
	return userDTO{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// login validates a username + password and mints a session token.
// Wrong credentials always return 401 with the same generic message so
// a probing attacker cannot enumerate accounts.
func (a *AdminServer) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	user, err := a.store.VerifyLogin(r.Context(), req.Username, req.Password)
	if errors.Is(err, store.ErrInvalidCredentials) {
		writeAdminError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sess, err := a.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.logger.Info("admin login", "user", user.Username)
	writeJSON(w, http.StatusOK, loginResponse{
		Token:     sess.Token,
		ExpiresAt: sess.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User:      toUserDTO(user),
	})
}

// logout revokes the caller's bearer token. Idempotent: a missing or
// already-expired token still returns 204.
func (a *AdminServer) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token != "" {
		_ = a.store.DeleteSession(r.Context(), token)
	}
	w.WriteHeader(http.StatusNoContent)
}

// me returns the currently authenticated user. The dashboard hits it
// on page load to bootstrap its "logged in as …" UI.
func (a *AdminServer) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r)
	if !ok {
		writeAdminError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// changePasswordRequest is the JSON shape posted to /admin/auth/password.
type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// changePassword rotates the authenticated user's own password. The
// caller must supply the current password — admin reset of someone
// else's password is intentionally not exposed in v2.2.
func (a *AdminServer) changePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r)
	if !ok {
		writeAdminError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if _, err := a.store.VerifyLogin(r.Context(), user.Username, req.OldPassword); err != nil {
		writeAdminError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	if err := a.store.UpdateAdminPassword(r.Context(), user.ID, req.NewPassword); err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.logger.Info("admin password rotated", "user", user.Username)
	w.WriteHeader(http.StatusNoContent)
}

// bearerToken extracts the raw token from an Authorization header.
// Returns "" when the header is missing or malformed.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// updateProfileRequest is the JSON shape posted to /admin/auth/profile.
type updateProfileRequest struct {
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// updateProfile saves changes to the caller's profile.
func (a *AdminServer) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r)
	if !ok {
		writeAdminError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if err := a.store.UpdateAdminProfile(r.Context(), user.ID, req.Nickname, req.Email, req.AvatarURL); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.logger.Info("admin profile updated", "user", user.Username)
	w.WriteHeader(http.StatusNoContent)
}

// uploadAvatar receives a multipart/form-data upload, saves the file to
// disk, and returns the path so the caller can set it as their avatar_url.
func (a *AdminServer) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r)
	if !ok {
		writeAdminError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// 5MB limit for avatar images
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeAdminError(w, http.StatusBadRequest, "failed to parse upload: "+err.Error())
		return
	}
	
	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "missing 'avatar' field in form")
		return
	}
	defer file.Close()

	// Ensure target directory exists natively without needing external scripts
	dir := "./data/avatars"
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "failed to configure upload directory")
		return
	}

	// Make the filename secure but deterministic based on user ID.
	// That way uploading a new avatar naturally overwrites the old one
	// without accumulating unbounded disk bytes.
	ext := ".png"
	if strings.HasSuffix(strings.ToLower(header.Filename), ".jpg") || strings.HasSuffix(strings.ToLower(header.Filename), ".jpeg") {
		ext = ".jpg"
	} else if strings.HasSuffix(strings.ToLower(header.Filename), ".webp") {
		ext = ".webp"
	} else if strings.HasSuffix(strings.ToLower(header.Filename), ".gif") {
		ext = ".gif"
	}
	filename := fmt.Sprintf("user_%d%s", user.ID, ext)
	outPath := filepath.Join(dir, filename)

	out, err := os.Create(outPath)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "failed to write file")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "failed to write file blobs")
		return
	}

	publicURL := "/admin/avatars/" + filename
	writeJSON(w, http.StatusOK, map[string]string{"avatar_url": publicURL})
}
