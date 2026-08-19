package repo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// The breaker's bugs were all concurrency bugs, and a fake store cannot
// show either of them: losing failures needs real concurrent writers,
// and admitting exactly one probe out of many needs a real row lock.
// These run against Postgres, like the quota tests and for the same
// reason.

const (
	testCooldown     = 30 * time.Second
	testProbeTimeout = 2 * time.Minute
	testThreshold    = 5
)

// seedProvider creates a provider (and its health row) under the org the
// test seeded.
func seedProvider(t *testing.T, pool *pgxpool.Pool, r Repo) string {
	t.Helper()
	ctx := context.Background()

	var orgID string
	if err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		t.Fatalf("seed provider: no org: %v", err)
	}
	var providerID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO providers (org_id, name, kind) VALUES ($1, 'p', 'openai_compatible')
		RETURNING id`, orgID).Scan(&providerID); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if _, err := r.GetProviderHealth(ctx, providerID); err != nil {
		t.Fatalf("seed health row: %v", err)
	}
	return providerID
}

func healthOf(t *testing.T, r Repo, providerID string) types.ProviderHealth {
	t.Helper()
	h, err := r.GetProviderHealth(context.Background(), providerID)
	if err != nil {
		t.Fatalf("read health: %v", err)
	}
	return h
}

// openBreaker trips the breaker and backdates opened_at so the cooldown
// has already elapsed.
func openBreaker(t *testing.T, pool *pgxpool.Pool, r Repo, providerID string) {
	t.Helper()
	ctx := context.Background()
	for range testThreshold {
		if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
			t.Fatalf("record failure: %v", err)
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE provider_health_states SET opened_at = now() - interval '1 hour' WHERE provider_id = $1`,
		providerID); err != nil {
		t.Fatalf("backdate opened_at: %v", err)
	}
}

func TestBreakerTripsAfterTheThreshold(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1) // seeds the org
	providerID := seedProvider(t, pool, r)

	for i := 1; i < testThreshold; i++ {
		if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
			t.Fatalf("failure %d: %v", i, err)
		}
		if got := healthOf(t, r, providerID); got.State != types.HealthStateNormal {
			t.Fatalf("state after %d failures = %q, want it still normal", i, got.State)
		}
	}

	if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
		t.Fatalf("threshold failure: %v", err)
	}
	got := healthOf(t, r, providerID)
	if got.State != types.HealthStateCircuitOpen {
		t.Errorf("state = %q, want circuit_open", got.State)
	}
	if got.OpenedAt == nil {
		t.Error("opened_at was not stamped")
	}
}

// The read-modify-write this replaced lost failures under exactly the
// conditions the breaker is for: a provider going down produces a burst
// of concurrent failures that all read the same count.
func TestConcurrentFailuresAreNotLost(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)

	const concurrent = 50
	var wg sync.WaitGroup
	for range concurrent {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A threshold nothing will reach, so the count is the only
			// thing under test.
			if err := r.RecordProviderFailure(ctx, providerID, 10_000); err != nil {
				t.Errorf("record failure: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := healthOf(t, r, providerID); got.ConsecutiveFailures != concurrent {
		t.Errorf("consecutive_failures = %d after %d concurrent failures, want %d",
			got.ConsecutiveFailures, concurrent, concurrent)
	}
}

func TestNormalProviderIsAlwaysAttemptable(t *testing.T) {
	pool, r := testPool(t)
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)

	for range 3 {
		ok, probe, err := r.ClaimProviderAttempt(context.Background(), providerID, testCooldown, testProbeTimeout)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if !ok {
			t.Fatal("a healthy provider was refused")
		}
		if probe {
			t.Error("attempting a healthy provider was reported as a probe claim")
		}
	}
	if got := healthOf(t, r, providerID); got.State != types.HealthStateNormal {
		t.Errorf("state = %q, want asking not to have changed it", got.State)
	}
}

func TestOpenBreakerRefusesUntilTheCooldownElapses(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)

	for range testThreshold {
		if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
			t.Fatalf("record failure: %v", err)
		}
	}
	ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if ok {
		t.Error("a provider that just tripped was attemptable inside its cooldown")
	}
}

// The half-open flood: the breaker is meant to let exactly one request
// through to see whether the provider recovered. The previous version
// returned true to every request that asked while half_open, so a
// provider that had just come back was met by the whole backlog at once.
func TestExactlyOneConcurrentProbeIsAdmitted(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)
	openBreaker(t, pool, r, providerID)

	const concurrent = 40
	admitted := make([]bool, concurrent)
	var wg sync.WaitGroup
	for i := range admitted {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, probe, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout)
			if err != nil {
				t.Errorf("claim %d: %v", i, err)
				return
			}
			if ok != probe {
				t.Errorf("claim %d: admitted=%v probe=%v; reaching an open breaker is only ever a probe", i, ok, probe)
			}
			admitted[i] = ok
		}(i)
	}
	wg.Wait()

	count := 0
	for _, ok := range admitted {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%d of %d concurrent requests were admitted as probes, want exactly 1", count, concurrent)
	}
	if got := healthOf(t, r, providerID); got.State != types.HealthStateHalfOpen {
		t.Errorf("state = %q, want half_open", got.State)
	}
}

func TestASuccessfulProbeClosesTheBreaker(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)
	openBreaker(t, pool, r, providerID)

	if ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); err != nil || !ok {
		t.Fatalf("probe not admitted: ok=%v err=%v", ok, err)
	}
	if err := r.RecordProviderSuccess(ctx, providerID); err != nil {
		t.Fatalf("record success: %v", err)
	}

	got := healthOf(t, r, providerID)
	if got.State != types.HealthStateNormal {
		t.Errorf("state = %q, want normal", got.State)
	}
	if got.ConsecutiveFailures != 0 || got.OpenedAt != nil {
		t.Errorf("health = %+v, want the failure count and opened_at cleared", got)
	}
}

func TestAFailedProbeReopensTheCircuitImmediately(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)
	openBreaker(t, pool, r, providerID)

	if ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); err != nil || !ok {
		t.Fatalf("probe not admitted: ok=%v err=%v", ok, err)
	}
	if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
		t.Fatalf("record failure: %v", err)
	}

	got := healthOf(t, r, providerID)
	if got.State != types.HealthStateCircuitOpen {
		t.Errorf("state = %q, want circuit_open again", got.State)
	}
	// And the fresh cooldown is running, so nothing gets straight back in.
	if ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); err != nil || ok {
		t.Errorf("a request got in right after a failed probe: ok=%v err=%v", ok, err)
	}
}

// A probe whose process died must not close the provider off forever:
// nothing else is allowed through to discover it recovered.
func TestALostProbeIsReleasedAfterItsTimeout(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)
	openBreaker(t, pool, r, providerID)

	if ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); err != nil || !ok {
		t.Fatalf("probe not admitted: ok=%v err=%v", ok, err)
	}
	// A second request while the probe is live gets nothing.
	if ok, _, _ := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); ok {
		t.Fatal("a second request was admitted while a probe was in flight")
	}

	if _, err := pool.Exec(ctx,
		`UPDATE provider_health_states SET last_probe_at = now() - interval '1 hour' WHERE provider_id = $1`,
		providerID); err != nil {
		t.Fatalf("backdate last_probe_at: %v", err)
	}

	ok, _, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !ok {
		t.Error("a provider was left closed off by a probe that never reported back")
	}
}

// A call admitted before the breaker tripped can still be in flight when
// it does. Letting its late success reopen the gate hands the whole
// backlog to a provider that just failed five times in a row.
func TestALateSuccessDoesNotReopenAnOpenCircuit(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)

	for range testThreshold {
		if err := r.RecordProviderFailure(ctx, providerID, testThreshold); err != nil {
			t.Fatalf("record failure: %v", err)
		}
	}
	if err := r.RecordProviderSuccess(ctx, providerID); err != nil {
		t.Fatalf("record success: %v", err)
	}

	if got := healthOf(t, r, providerID); got.State != types.HealthStateCircuitOpen {
		t.Errorf("state = %q, want the breaker to have stayed open", got.State)
	}
}

// A probe claimed by a request that was then refused downstream -- out
// of budget, model out of scope -- has to go back. Otherwise a key with
// no budget left holds every provider it routes to shut until the probe
// times out.
func TestAnAbandonedProbeCanBeHandedBack(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)
	openBreaker(t, pool, r, providerID)

	if ok, probe, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout); err != nil || !ok || !probe {
		t.Fatalf("probe not claimed: ok=%v probe=%v err=%v", ok, probe, err)
	}
	if err := r.ReleaseProviderProbe(ctx, providerID); err != nil {
		t.Fatalf("release: %v", err)
	}

	if got := healthOf(t, r, providerID); got.State != types.HealthStateCircuitOpen {
		t.Errorf("state = %q, want circuit_open again", got.State)
	}

	// And the next request can take the slot straight away: the provider
	// was never actually tried, so it should not sit out another cooldown.
	ok, probe, err := r.ClaimProviderAttempt(ctx, providerID, testCooldown, testProbeTimeout)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !ok || !probe {
		t.Errorf("ok=%v probe=%v, want the released slot immediately claimable", ok, probe)
	}
}

// Releasing when no probe is outstanding must not disturb a healthy
// provider.
func TestReleasingProbeIsHarmlessWhenNoneIsOutstanding(t *testing.T) {
	pool, r := testPool(t)
	seedKey(t, pool, 1)
	providerID := seedProvider(t, pool, r)

	if err := r.ReleaseProviderProbe(context.Background(), providerID); err != nil {
		t.Fatalf("release: %v", err)
	}
	if got := healthOf(t, r, providerID); got.State != types.HealthStateNormal {
		t.Errorf("state = %q, want a healthy provider left alone", got.State)
	}
}
