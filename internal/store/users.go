// users.go — admin user accounts and login sessions for the dashboard.
//
// Starting with v2.2 the admin surface is no longer guarded by a static
// FLUXA_MASTER_KEY env var. Operators sign in with a username and a
// bcrypt-hashed password, exchanging credentials for an opaque session
// token that lives in the admin_sessions table. Tokens are random 32-byte
// hex strings with a configurable TTL (one week by default), refreshed on
// every successful auth check.
//
// Keeping the auth state in SQLite means a redeploy never logs anyone
// out, multi-replica setups can share a single users table, and
// passwords can be rotated through the same admin REST surface as the
// rest of the gateway.

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned by VerifyLogin when either the
// username does not exist or the password hash does not match. The two
// cases share an error so a probing attacker cannot enumerate accounts.
var ErrInvalidCredentials = errors.New("store: invalid credentials")

// DefaultSessionTTL is how long a freshly minted session token is valid
// before the owner has to log in again. One week is short enough that a
// stolen browser cookie expires on its own and long enough that
// operators are not nagged daily.
const DefaultSessionTTL = 7 * 24 * time.Hour

// AdminUser is one row of the admin_users table. Password hashes are
// never serialised over the wire — handlers strip them before they hand
// the struct back to the API layer.
type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AdminSession represents a live login token. The token itself is the
// primary key so requireAuth can look it up in O(1) without a join.
type AdminSession struct {
	Token     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// hashPassword runs bcrypt at the default cost. Cost 10 is the library
// default and roughly matches "100ms on a laptop" — slow enough to
// frustrate brute force, fast enough to keep the login endpoint snappy.
func hashPassword(plain string) (string, error) {
	if len(plain) < 4 {
		return "", errors.New("password too short (min 4 chars)")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CountAdminUsers returns the number of rows in admin_users. main.go
// uses it to decide whether to seed the default admin account on first
// boot.
func (s *Store) CountAdminUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n, err
}

// CreateAdminUser inserts a new account, hashing the supplied plaintext
// password before persisting. The username is normalised to lower-case
// so logins are case-insensitive without needing a collation.
func (s *Store) CreateAdminUser(ctx context.Context, username, password string) (AdminUser, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return AdminUser{}, errors.New("username is required")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return AdminUser{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_users (username, password_hash) VALUES (?, ?)`,
		username, hash,
	)
	if err != nil {
		return AdminUser{}, fmt.Errorf("create admin user: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetAdminUserByID(ctx, id)
}

// GetAdminUserByID loads one user row by its primary key.
func (s *Store) GetAdminUserByID(ctx context.Context, id int64) (AdminUser, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE id = ?`, id)
	return scanAdminUser(row)
}

// GetAdminUserByUsername loads one user row by username (case-insensitive).
func (s *Store) GetAdminUserByUsername(ctx context.Context, username string) (AdminUser, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE username = ?`, username)
	return scanAdminUser(row)
}

// VerifyLogin checks a username + plaintext password against the store
// and returns the matching user on success. Wrong username and wrong
// password collapse to the same ErrInvalidCredentials so callers cannot
// time-side-channel which one was wrong.
func (s *Store) VerifyLogin(ctx context.Context, username, password string) (AdminUser, error) {
	user, err := s.GetAdminUserByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		// Run a dummy bcrypt anyway so the timing of "no such user"
		// matches "wrong password" for the same username.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvali"), []byte(password))
		return AdminUser{}, ErrInvalidCredentials
	}
	if err != nil {
		return AdminUser{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AdminUser{}, ErrInvalidCredentials
	}
	return user, nil
}

// UpdateAdminPassword rotates one user's password to a new bcrypt hash.
// Callers should verify the old password first when this is a "change my
// own password" flow rather than an admin reset.
func (s *Store) UpdateAdminPassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE admin_users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		hash, userID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	// Invalidate every existing session for this user so a stolen token
	// is useless after a password rotation.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE user_id = ?`, userID)
	return nil
}

// CreateSession mints a fresh opaque token for the given user and
// stores it with an expiry of DefaultSessionTTL from now.
func (s *Store) CreateSession(ctx context.Context, userID int64) (AdminSession, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return AdminSession{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().Add(DefaultSessionTTL).UTC()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires,
	); err != nil {
		return AdminSession{}, err
	}
	return AdminSession{
		Token:     token,
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expires,
	}, nil
}

// LookupSession resolves a token to its owning user. Expired tokens are
// deleted on the way out so the table self-cleans without a separate
// cron job. Returns ErrInvalidCredentials when the token is unknown or
// expired.
func (s *Store) LookupSession(ctx context.Context, token string) (AdminUser, error) {
	if token == "" {
		return AdminUser{}, ErrInvalidCredentials
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.created_at, u.updated_at, sess.expires_at
		FROM admin_sessions sess
		JOIN admin_users u ON u.id = sess.user_id
		WHERE sess.token = ?`, token)
	var (
		user      AdminUser
		expiresAt time.Time
	)
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminUser{}, ErrInvalidCredentials
		}
		return AdminUser{}, err
	}
	if time.Now().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token = ?`, token)
		return AdminUser{}, ErrInvalidCredentials
	}
	return user, nil
}

// DeleteSession revokes one specific token (logout flow).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token = ?`, token)
	return err
}

// PurgeExpiredSessions deletes every session whose expiry has passed.
// main.go calls it once at boot so a long-idle deployment does not
// accumulate stale rows.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at < CURRENT_TIMESTAMP`)
	return err
}

func scanAdminUser(sc scanner) (AdminUser, error) {
	var u AdminUser
	if err := sc.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminUser{}, ErrNotFound
		}
		return AdminUser{}, err
	}
	return u, nil
}
