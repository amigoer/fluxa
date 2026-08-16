package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionRepo stores the server-side sessions the console logs in with.
// Revocation is the reason this lives in Postgres at all rather than in
// a stateless token (DESIGN.md 7.1).
type SessionRepo interface {
	CreateSession(ctx context.Context, tokenHash, memberID string, expiresAt time.Time) error
	GetSession(ctx context.Context, tokenHash string) (SessionRecord, error)
	RevokeSession(ctx context.Context, tokenHash string) error
	RevokeAllSessionsForMember(ctx context.Context, memberID string) error
}

// SessionRecord is the row shape sessions are stored/read as; token_hash
// is sha256(raw token), never the raw token itself (DESIGN.md 7.1).
type SessionRecord struct {
	TokenHash string
	MemberID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (r *repo) CreateSession(ctx context.Context, tokenHash, memberID string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, member_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, memberID, expiresAt,
	)
	return err
}

func (r *repo) GetSession(ctx context.Context, tokenHash string) (SessionRecord, error) {
	var s SessionRecord
	err := r.pool.QueryRow(ctx,
		`SELECT token_hash, member_id, expires_at, revoked_at FROM sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&s.TokenHash, &s.MemberID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrNotFound
	}
	return s, err
}

func (r *repo) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}

// RevokeAllSessionsForMember lets an admin force a member's active
// sessions to end immediately -- the whole point of choosing a
// server-side session over a stateless JWT (DESIGN.md 7.1).
func (r *repo) RevokeAllSessionsForMember(ctx context.Context, memberID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE member_id = $1 AND revoked_at IS NULL`,
		memberID,
	)
	return err
}
