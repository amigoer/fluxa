package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// IdentityRepo stores both halves of external sign-in: the per-provider
// app credentials, and the link from an external account to a member.
type IdentityRepo interface {
	FindMemberByExternalIdentity(ctx context.Context, provider types.IdentityProvider, externalUserID string) (types.Member, error)
	LinkExternalIdentity(ctx context.Context, memberID string, provider types.IdentityProvider, externalUserID string) error
	GetIdentityConfig(ctx context.Context, provider types.IdentityProvider) (types.IdentityConfig, error)
	UpsertIdentityConfig(ctx context.Context, c types.IdentityConfig) error
}

func (r *repo) FindMemberByExternalIdentity(ctx context.Context, provider types.IdentityProvider, externalUserID string) (types.Member, error) {
	var m types.Member
	err := r.pool.QueryRow(ctx, `
		SELECT m.id, m.org_id, m.department_id, m.role_id, m.name, m.email, m.phone, m.status, m.created_at
		FROM members m
		JOIN external_identities ei ON ei.member_id = m.id
		WHERE ei.provider = $1 AND ei.external_user_id = $2`,
		provider, externalUserID,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Member{}, ErrNotFound
	}
	return m, err
}

func (r *repo) LinkExternalIdentity(ctx context.Context, memberID string, provider types.IdentityProvider, externalUserID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO external_identities (member_id, provider, external_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, external_user_id) DO NOTHING`,
		memberID, provider, externalUserID,
	)
	return err
}

func (r *repo) GetIdentityConfig(ctx context.Context, provider types.IdentityProvider) (types.IdentityConfig, error) {
	var c types.IdentityConfig
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, app_id, app_secret, callback_path, enabled, created_at, updated_at
		FROM identity_configs WHERE provider = $1`, provider,
	).Scan(&c.ID, &c.Provider, &c.AppID, &c.AppSecret, &c.CallbackPath, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.IdentityConfig{}, ErrNotFound
	}
	return c, err
}

func (r *repo) UpsertIdentityConfig(ctx context.Context, c types.IdentityConfig) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO identity_configs (provider, app_id, app_secret, callback_path, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (provider) DO UPDATE SET
			app_id = EXCLUDED.app_id,
			app_secret = EXCLUDED.app_secret,
			callback_path = EXCLUDED.callback_path,
			enabled = EXCLUDED.enabled,
			updated_at = now()`,
		c.Provider, c.AppID, c.AppSecret, c.CallbackPath, c.Enabled,
	)
	return err
}
