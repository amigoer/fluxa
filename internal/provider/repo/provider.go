package repo

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ProviderRepo stores the upstream vendors an org has configured, with
// their credentials in the JSON config column.
type ProviderRepo interface {
	CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error)
	GetProvider(ctx context.Context, id string) (types.Provider, error)
	ListProviders(ctx context.Context, orgID string) ([]types.Provider, error)
}

func (r *repo) CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error) {
	raw, err := json.Marshal(p.Config)
	if err != nil {
		return types.Provider{}, err
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO providers (org_id, name, kind, config, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		p.OrgID, p.Name, p.Kind, raw, p.Status,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *repo) GetProvider(ctx context.Context, id string) (types.Provider, error) {
	var p types.Provider
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, name, kind, config, status, created_at, updated_at
		FROM providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Kind, &raw, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Provider{}, ErrNotFound
	}
	if err != nil {
		return types.Provider{}, err
	}
	if err := json.Unmarshal(raw, &p.Config); err != nil {
		return types.Provider{}, err
	}
	return p, nil
}

func (r *repo) ListProviders(ctx context.Context, orgID string) ([]types.Provider, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, org_id, name, kind, config, status, created_at, updated_at
		FROM providers WHERE org_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Provider
	for rows.Next() {
		var p types.Provider
		var raw []byte
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Kind, &raw, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &p.Config); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
