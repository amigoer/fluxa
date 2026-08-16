package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// AuthSettingsRepo stores the deployment-wide login policy: whether
// local accounts may register at all, and whether they need approval.
type AuthSettingsRepo interface {
	GetAuthSettings(ctx context.Context) (types.AuthSettings, error)
	UpdateAuthSettings(ctx context.Context, s types.AuthSettings) error
}

func (r *repo) GetAuthSettings(ctx context.Context) (types.AuthSettings, error) {
	var s types.AuthSettings
	err := r.pool.QueryRow(ctx,
		`SELECT local_account_enabled, local_account_requires_approval FROM auth_settings`,
	).Scan(&s.LocalAccountEnabled, &s.LocalAccountRequiresApproval)
	return s, err
}

func (r *repo) UpdateAuthSettings(ctx context.Context, s types.AuthSettings) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth_settings SET local_account_enabled = $1, local_account_requires_approval = $2`,
		s.LocalAccountEnabled, s.LocalAccountRequiresApproval,
	)
	return err
}
