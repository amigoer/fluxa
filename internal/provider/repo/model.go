package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ModelRepo stores the models exposed on top of a provider, with the
// pricing routing uses to estimate a call's cost.
type ModelRepo interface {
	CreateModel(ctx context.Context, m types.Model) (types.Model, error)
	GetModel(ctx context.Context, id string) (types.Model, error)
	ListModels(ctx context.Context, orgID string) ([]types.Model, error)
	ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error)
}

func (r *repo) CreateModel(ctx context.Context, m types.Model) (types.Model, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO models (provider_id, name, model_identifier, status, input_price_cents_per_1m, output_price_cents_per_1m, context_window)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		m.ProviderID, m.Name, m.ModelIdentifier, m.Status, m.InputPriceCentsPer1M, m.OutputPriceCentsPer1M, m.ContextWindow,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (r *repo) GetModel(ctx context.Context, id string) (types.Model, error) {
	var m types.Model
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider_id, name, model_identifier, status, input_price_cents_per_1m, output_price_cents_per_1m, context_window, created_at, updated_at
		FROM models WHERE id = $1`, id,
	).Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Model{}, ErrNotFound
	}
	return m, err
}

func (r *repo) ListModels(ctx context.Context, orgID string) ([]types.Model, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.provider_id, m.name, m.model_identifier, m.status, m.input_price_cents_per_1m, m.output_price_cents_per_1m, m.context_window, m.created_at, m.updated_at, p.kind
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.org_id = $1
		ORDER BY m.created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Model
	for rows.Next() {
		var m types.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt, &m.ProviderKind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *repo) ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.provider_id, m.name, m.model_identifier, m.status, m.input_price_cents_per_1m, m.output_price_cents_per_1m, m.context_window, m.created_at, m.updated_at, p.kind
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.org_id = $1 AND m.status = 'published' AND p.status = 'active'
		ORDER BY m.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Model
	for rows.Next() {
		var m types.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt, &m.ProviderKind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
