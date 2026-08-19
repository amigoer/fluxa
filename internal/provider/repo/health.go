package repo

import (
	"context"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// HealthRepo persists each provider's circuit breaker state, so a
// tripped breaker survives a restart instead of letting every replica
// rediscover a dead provider the hard way.
//
// Every state transition is a single guarded statement rather than a
// read, a decision in Go, and a write back. The read-modify-write it
// replaced lost failures under exactly the conditions the breaker exists
// for: a provider going down produces a burst of concurrent failures,
// every one of them reads the same count, and the counter creeps up by
// one per burst instead of one per failure -- so the threshold could
// take a very long time to be reached, or never be.
type HealthRepo interface {
	GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error)
	ClaimProviderAttempt(ctx context.Context, providerID string, cooldown, probeTimeout time.Duration) (allowed, probe bool, err error)
	RecordProviderFailure(ctx context.Context, providerID string, failureThreshold int) error
	RecordProviderSuccess(ctx context.Context, providerID string) error
	ReleaseProviderProbe(ctx context.Context, providerID string) error
}

// GetProviderHealth reads a provider's breaker state, creating the row
// on first sight. It serves the admin console and the seeding done when
// a provider is created; the gateway's hot path never reads through it,
// because a decision made from a read is a decision made from a stale
// value.
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

// ClaimProviderAttempt decides in one statement whether a request may be
// sent to providerID, and -- when the answer is "one probe may" --
// claims that probe for the caller.
//
// The claim is the point. A breaker in half_open is meant to let exactly
// one request through to see whether the provider recovered; the
// previous implementation returned true for every request that asked
// while half_open, so a provider that had just come back was met by the
// whole backlog at once. Here the transition into half_open *is* the
// admission: the UPDATE's WHERE is re-evaluated against the committed
// row, so of N concurrent callers exactly one finds a circuit_open row
// to move and the rest find it already moved.
//
// probeTimeout releases a probe that never reported back -- a process
// killed mid-call. Without it one lost probe would leave the provider
// closed off permanently, since nothing else is allowed through to
// discover it recovered.
func (r *repo) ClaimProviderAttempt(ctx context.Context, providerID string, cooldown, probeTimeout time.Duration) (allowed, probe bool, err error) {
	err = r.pool.QueryRow(ctx, `
		WITH before AS (
			SELECT state FROM provider_health_states WHERE provider_id = $1
		), claimed AS (
			UPDATE provider_health_states
			SET state = 'half_open', last_probe_at = now(), updated_at = now()
			WHERE provider_id = $1
			  AND (
				(state = 'circuit_open' AND opened_at IS NOT NULL AND opened_at < now() - $2::interval)
				OR (state = 'half_open' AND (last_probe_at IS NULL OR last_probe_at < now() - $3::interval))
			  )
			RETURNING 1
		)
		SELECT
			COALESCE((SELECT state FROM before) = 'normal', true) OR EXISTS (SELECT 1 FROM claimed),
			EXISTS (SELECT 1 FROM claimed)`,
		providerID, cooldown.String(), probeTimeout.String(),
	).Scan(&allowed, &probe)
	return allowed, probe, err
}

// RecordProviderFailure counts a failure and trips the breaker when that
// is what the count now means: a failed probe reopens the circuit
// immediately, and enough consecutive failures from normal open it for
// the first time.
func (r *repo) RecordProviderFailure(ctx context.Context, providerID string, failureThreshold int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_health_states SET
			consecutive_failures = consecutive_failures + 1,
			state = CASE
				WHEN state = 'half_open' THEN 'circuit_open'
				WHEN state = 'normal' AND consecutive_failures + 1 >= $2 THEN 'circuit_open'
				ELSE state
			END,
			opened_at = CASE
				WHEN state = 'half_open' THEN now()
				WHEN state = 'normal' AND consecutive_failures + 1 >= $2 THEN now()
				ELSE opened_at
			END,
			updated_at = now()
		WHERE provider_id = $1`,
		providerID, failureThreshold,
	)
	return err
}

// RecordProviderSuccess clears the failure count and closes the breaker.
//
// It deliberately does nothing to a circuit_open provider. A call
// admitted before the breaker tripped can still be in flight when it
// does, and letting its late success reopen the gate would hand the
// whole backlog straight back to a provider that had just failed five
// times in a row -- the exact flood the cooldown exists to prevent. The
// probe is how a circuit_open provider gets to prove it recovered, and
// by then its state is half_open, which this does close.
func (r *repo) RecordProviderSuccess(ctx context.Context, providerID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_health_states SET
			state = 'normal',
			consecutive_failures = 0,
			opened_at = NULL,
			last_probe_at = NULL,
			updated_at = now()
		WHERE provider_id = $1 AND state <> 'circuit_open'`,
		providerID,
	)
	return err
}

// ReleaseProviderProbe hands back a probe slot that was claimed but
// never used, because the request that claimed it was refused further
// down the pipeline -- out of budget, model out of the key's scope.
//
// opened_at is deliberately left where it was. It is already older than
// the cooldown, which is how the probe came to be claimed, so the next
// request can claim one straight away. That is the right answer: the
// provider was never actually tried, so making it wait out another
// cooldown would punish it for something that happened on this side.
func (r *repo) ReleaseProviderProbe(ctx context.Context, providerID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_health_states
		SET state = 'circuit_open', last_probe_at = NULL, updated_at = now()
		WHERE provider_id = $1 AND state = 'half_open'`, providerID)
	return err
}
