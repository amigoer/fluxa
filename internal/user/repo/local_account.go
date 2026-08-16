package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// LocalAccountRepo stores phone/email accounts for orgs that don't sign
// in through an external identity source. There is no password column:
// authentication is by one-time code (see OTPRepo).
type LocalAccountRepo interface {
	CreateLocalAccount(ctx context.Context, a types.LocalAccount) (types.LocalAccount, error)
	FindLocalAccountByIdentifier(ctx context.Context, identifier string) (types.LocalAccount, error)
}

func (r *repo) CreateLocalAccount(ctx context.Context, a types.LocalAccount) (types.LocalAccount, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO local_accounts (member_id, phone, email)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		a.MemberID, a.Phone, a.Email,
	).Scan(&a.ID, &a.CreatedAt)
	return a, err
}

func (r *repo) FindLocalAccountByIdentifier(ctx context.Context, identifier string) (types.LocalAccount, error) {
	var a types.LocalAccount
	err := r.pool.QueryRow(ctx, `
		SELECT id, member_id, phone, email, created_at
		FROM local_accounts WHERE phone = $1 OR email = $1`, identifier,
	).Scan(&a.ID, &a.MemberID, &a.Phone, &a.Email, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.LocalAccount{}, ErrNotFound
	}
	return a, err
}
