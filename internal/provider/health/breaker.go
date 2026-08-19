// Package health implements the provider circuit breaker state machine
// from DESIGN.md: normal -> circuit_open after too many consecutive
// failures -> half_open after a cooldown to probe recovery -> back to
// normal on a successful probe, or circuit_open again on a failed one.
// This is what keeps a single misbehaving provider from taking down
// every request that happens to route to it.
//
// The state machine lives in SQL rather than here (see repo.HealthRepo).
// That is not an implementation detail: every transition is decided by
// the same concurrent traffic the breaker is supposed to be protecting
// against, so each one has to be a single guarded statement. Deciding in
// Go from a value read a moment earlier is how the previous version lost
// failures under load and let the entire backlog through a half-open
// breaker at once.
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

	// probeTimeout releases a probe that never reported its outcome,
	// which happens when the process running it dies mid-call.
	//
	// It bounds a lost probe rather than a slow one. A genuinely slow
	// probe that outlives it means a second probe goes out alongside the
	// first, which costs one extra request; a lost probe with no timeout
	// means the provider is closed off permanently, because nothing else
	// is allowed through to discover it recovered.
	probeTimeout = 2 * time.Minute
)

type Store interface {
	GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error)
	ClaimProviderAttempt(ctx context.Context, providerID string, cooldown, probeTimeout time.Duration) (allowed, probe bool, err error)
	RecordProviderFailure(ctx context.Context, providerID string, failureThreshold int) error
	RecordProviderSuccess(ctx context.Context, providerID string) error
	ReleaseProviderProbe(ctx context.Context, providerID string) error
}

type Breaker struct {
	store Store
}

func NewBreaker(store Store) *Breaker {
	return &Breaker{store: store}
}

// CanAttempt reports whether a request may be routed to providerID right
// now, and whether saying yes cost the provider its single probe slot.
//
// It is not a question with a side-effect-free answer: a circuit_open
// provider whose cooldown has elapsed becomes attemptable by exactly one
// caller, and saying yes to that caller is the same act as claiming the
// probe. Every other caller is told no until the probe reports back --
// which is why a caller that is told yes with probe set owes the breaker
// an outcome, either a recorded result or a ReleaseProbe.
func (b *Breaker) CanAttempt(ctx context.Context, providerID string) (allowed, probe bool, err error) {
	return b.store.ClaimProviderAttempt(ctx, providerID, cooldown, probeTimeout)
}

// RecordSuccess clears the failure count and closes the breaker.
func (b *Breaker) RecordSuccess(ctx context.Context, providerID string) error {
	return b.store.RecordProviderSuccess(ctx, providerID)
}

// RecordFailure counts a failure and trips the breaker when the count
// says to.
func (b *Breaker) RecordFailure(ctx context.Context, providerID string) error {
	return b.store.RecordProviderFailure(ctx, providerID, failureThreshold)
}

// ReleaseProbe gives back a probe slot claimed by a request that never
// reached the provider. Without it, a key that is simply out of budget
// would burn the probe slot of every provider it routed to and hold each
// one shut until the probe timed out.
func (b *Breaker) ReleaseProbe(ctx context.Context, providerID string) error {
	return b.store.ReleaseProviderProbe(ctx, providerID)
}
