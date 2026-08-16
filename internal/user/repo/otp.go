package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// OTPRepo is the durable half of one-time code auth: issuing codes,
// rate limiting them, and spending them exactly once.
type OTPRepo interface {
	CountOTPsSince(ctx context.Context, identifier string, since time.Time) (int, time.Time, error)
	CreateOTP(ctx context.Context, identifier string, purpose types.OTPPurpose, codeHash string, expiresAt time.Time) error
	ConsumeOTP(ctx context.Context, identifier string, purpose types.OTPPurpose, codeHash string) (bool, error)
}

// maxOTPAttempts is how many guesses one code will tolerate before it is
// spent. Five is comfortably more than a person mistypes and nowhere near
// enough to search a six-digit space.
const maxOTPAttempts = 5

// CreateOTP stores a new one-time code, hashed, for identifier/purpose.
// CountOTPsSince reports how many codes were issued to identifier since
// `since`, and when the most recent one went out. Both come from one
// query because the caller needs both to answer "may I send another?".
//
// This is the durable half of the OTP rate limit: it survives a restart
// and is shared across replicas, which the in-process limiter is not.
func (r *repo) CountOTPsSince(ctx context.Context, identifier string, since time.Time) (int, time.Time, error) {
	var count int
	var latest *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT count(*), max(created_at) FROM local_otp_codes
		WHERE identifier = $1 AND created_at >= $2`,
		identifier, since,
	).Scan(&count, &latest)
	if err != nil {
		return 0, time.Time{}, err
	}
	if latest == nil {
		return count, time.Time{}, nil
	}
	return count, *latest, nil
}

func (r *repo) CreateOTP(ctx context.Context, identifier string, purpose types.OTPPurpose, codeHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO local_otp_codes (identifier, purpose, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		identifier, purpose, codeHash, expiresAt,
	)
	return err
}

// ConsumeOTP atomically marks the newest live code for identifier/purpose
// as consumed if its hash matches, returning whether it matched. Using
// UPDATE ... RETURNING makes the check-and-consume a single round trip so
// two concurrent attempts can't both succeed.
//
// The row is selected without matching the hash, and every attempt --
// right or wrong -- increments the counter, which is what makes a wrong
// guess cost something. Selecting by hash instead (as this did before)
// meant a wrong guess simply found no row and could be repeated forever.
func (r *repo) ConsumeOTP(ctx context.Context, identifier string, purpose types.OTPPurpose, codeHash string) (bool, error) {
	var matched bool
	err := r.pool.QueryRow(ctx, `
		UPDATE local_otp_codes
		SET attempts = attempts + 1,
		    consumed_at = CASE WHEN code_hash = $3 THEN now() ELSE consumed_at END
		WHERE id = (
			SELECT id FROM local_otp_codes
			WHERE identifier = $1 AND purpose = $2
			  AND consumed_at IS NULL AND expires_at > now()
			  AND attempts < $4
			ORDER BY created_at DESC
			LIMIT 1
		)
		RETURNING code_hash = $3`,
		identifier, purpose, codeHash, maxOTPAttempts,
	).Scan(&matched)
	if errors.Is(err, pgx.ErrNoRows) {
		// No live code left: never issued, already used, expired, or its
		// attempts are spent. All of these are "invalid code" to the
		// caller -- distinguishing them would say more than it should.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return matched, nil
}
