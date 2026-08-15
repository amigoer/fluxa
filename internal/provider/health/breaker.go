// Package health implements the provider circuit breaker state machine
// from DESIGN.md: normal -> circuit_open after too many consecutive
// failures -> half_open after a cooldown to probe recovery -> back to
// normal on a successful probe, or circuit_open again on a failed one.
// This is what keeps a single misbehaving provider from taking down
// every request that happens to route to it.
package health

import (
	"context"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

const (
	// failureThreshold is how many consecutive failures trip the
	// breaker from normal to circuit_open.
	failureThreshold = 5

	// cooldown is how long the breaker stays circuit_open before
	// letting one probe request through as half_open.
	cooldown = 30 * time.Second
)

type Store interface {
	GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error)
	SaveProviderHealth(ctx context.Context, h types.ProviderHealth) error
}

type Breaker struct {
	store Store
}

func NewBreaker(store Store) *Breaker {
	return &Breaker{store: store}
}

// CanAttempt reports whether a request may be routed to providerID right
// now. A circuit_open provider becomes attemptable again -- transitioning
// to half_open -- once the cooldown has elapsed, so the gateway can send
// exactly one probe request through it and see whether it recovered.
func (b *Breaker) CanAttempt(ctx context.Context, providerID string) (bool, error) {
	h, err := b.store.GetProviderHealth(ctx, providerID)
	if err != nil {
		return false, err
	}

	switch h.State {
	case types.HealthStateNormal, types.HealthStateHalfOpen:
		return true, nil
	case types.HealthStateCircuitOpen:
		if h.OpenedAt != nil && time.Since(*h.OpenedAt) >= cooldown {
			now := time.Now()
			h.State = types.HealthStateHalfOpen
			h.LastProbeAt = &now
			return true, b.store.SaveProviderHealth(ctx, h)
		}
		return false, nil
	default:
		return true, nil
	}
}

// RecordSuccess clears the failure count and, if the provider was being
// probed in half_open, closes the breaker back to normal.
func (b *Breaker) RecordSuccess(ctx context.Context, providerID string) error {
	h, err := b.store.GetProviderHealth(ctx, providerID)
	if err != nil {
		return err
	}
	h.State = types.HealthStateNormal
	h.ConsecutiveFailures = 0
	h.OpenedAt = nil
	return b.store.SaveProviderHealth(ctx, h)
}

// RecordFailure increments the failure count and trips the breaker: a
// failed half_open probe reopens the circuit immediately, and enough
// consecutive failures from normal opens it for the first time.
func (b *Breaker) RecordFailure(ctx context.Context, providerID string) error {
	h, err := b.store.GetProviderHealth(ctx, providerID)
	if err != nil {
		return err
	}

	h.ConsecutiveFailures++
	now := time.Now()

	switch h.State {
	case types.HealthStateHalfOpen:
		h.State = types.HealthStateCircuitOpen
		h.OpenedAt = &now
	case types.HealthStateNormal:
		if h.ConsecutiveFailures >= failureThreshold {
			h.State = types.HealthStateCircuitOpen
			h.OpenedAt = &now
		}
	}

	return b.store.SaveProviderHealth(ctx, h)
}
