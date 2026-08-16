package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// HealthRepo persists each provider's circuit breaker state, so a
// tripped breaker survives a restart instead of letting every replica
// rediscover a dead provider the hard way.
type HealthRepo interface {
	GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error)
	SaveProviderHealth(ctx context.Context, h types.ProviderHealth) error
}

func (r *repo) GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error) {
	var h types.ProviderHealth
	err := r.pool.QueryRow(ctx, `
		INSERT INTO provider_health_states (provider_id) VALUES ($1)
		ON CONFLICT (provider_id) DO UPDATE SET provider_id = EXCLUDED.provider_id
		RETURNING provider_id, state, consecutive_failures, opened_at, last_probe_at, updated_at`,
		providerID,
	).Scan(&h.ProviderID, &h.State, &h.ConsecutiveFailures, &h.OpenedAt, &h.LastProbeAt, &h.UpdatedAt)
	return h, err
}

func (r *repo) SaveProviderHealth(ctx context.Context, h types.ProviderHealth) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_health_states SET
			state = $1, consecutive_failures = $2, opened_at = $3, last_probe_at = $4, updated_at = now()
		WHERE provider_id = $5`,
		h.State, h.ConsecutiveFailures, h.OpenedAt, h.LastProbeAt, h.ProviderID,
	)
	return err
}
