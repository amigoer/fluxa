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
	ListModelsForVirtualKey(ctx context.Context, virtualKeyID string) ([]types.Model, error)
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

// ListModelsForVirtualKey returns the published models a given key may
// actually call: the org's published catalogue, narrowed to the key's
// model_scope when it has one.
//
// The gateway asks by key rather than by org because that is all a
// bearer token identifies -- there is no session and no principal on
// that path -- and because "what may this key call" is the question
// /v1/models is really being asked, not "what exists".
func (r *repo) ListModelsForVirtualKey(ctx context.Context, virtualKeyID string) ([]types.Model, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.provider_id, m.name, m.model_identifier, m.status,
		       m.input_price_cents_per_1m, m.output_price_cents_per_1m, m.context_window,
		       m.created_at, m.updated_at, p.kind
		FROM virtual_keys vk
		LEFT JOIN members mem ON mem.id = vk.owner_member_id
		LEFT JOIN departments d ON d.id = vk.owner_department_id
		JOIN providers p ON p.org_id = COALESCE(mem.org_id, d.org_id)
		JOIN models m ON m.provider_id = p.id
		WHERE vk.id = $1
		  AND m.status = 'published'
		  AND p.status = 'active'
		  AND (vk.model_scope IS NULL OR m.id = ANY (vk.model_scope))
		ORDER BY m.name`, virtualKeyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Model
	for rows.Next() {
		var m types.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status,
			&m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow,
			&m.CreatedAt, &m.UpdatedAt, &m.ProviderKind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
