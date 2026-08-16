package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// OrganizationRepo stores the single organization this deployment
// serves. GetOrganization returning ErrNotFound is what the first-run
// setup flow keys off of.
type OrganizationRepo interface {
	GetOrganization(ctx context.Context) (types.Organization, error)
	CreateOrganization(ctx context.Context, name string) (types.Organization, error)
}

func (r *repo) GetOrganization(ctx context.Context) (types.Organization, error) {
	var o types.Organization
	err := r.pool.QueryRow(ctx, `SELECT id, name, created_at FROM organizations LIMIT 1`).
		Scan(&o.ID, &o.Name, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Organization{}, ErrNotFound
	}
	return o, err
}

func (r *repo) CreateOrganization(ctx context.Context, name string) (types.Organization, error) {
	var o types.Organization
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, name, created_at`, name,
	).Scan(&o.ID, &o.Name, &o.CreatedAt)
	return o, err
}
