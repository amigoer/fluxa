package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/platform/db"
	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/types"
)

// Quota is the one place in this codebase where being wrong costs the
// operator real money, and the behaviour that matters most -- concurrent
// calls not each being told there is room for the same budget -- cannot
// be shown against a fake. These run against a real Postgres.
//
//	FLUXA_TEST_DATABASE_URL=postgres://fluxa:fluxa@localhost:5432/postgres?sslmode=disable go test ./internal/provider/repo/
func testPool(t *testing.T) (*pgxpool.Pool, Repo) {
	t.Helper()

	admin := os.Getenv("FLUXA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("set FLUXA_TEST_DATABASE_URL to run the quota reservation tests")
	}

	ctx := context.Background()
	adminPool, err := db.Open(ctx, admin)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer adminPool.Close()

	name := fmt.Sprintf("fluxa_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	dsn := replaceDBName(admin, name)
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanupPool, err := db.Open(context.Background(), admin)
		if err != nil {
			return
		}
		defer cleanupPool.Close()
		_, _ = cleanupPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	return pool, New(pool)
}

func replaceDBName(dsn, name string) string {
	// postgres://user:pass@host:port/dbname?params
	slash := len(dsn) - 1
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			slash = i
			break
		}
	}
	rest := dsn[slash+1:]
	query := ""
	for i, c := range rest {
		if c == '?' {
			query = rest[i:]
			break
		}
	}
	return dsn[:slash+1] + name + query
}

// seedKey creates the org/role/member a virtual key needs to exist, and
// the key itself with the given budget.
func seedKey(t *testing.T, pool *pgxpool.Pool, budgetMicroCents int64) string {
	t.Helper()
	ctx := context.Background()

	var orgID, roleID, memberID string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO organizations (name) VALUES ('Acme') RETURNING id`).Scan(&orgID))
	must(pool.QueryRow(ctx, `INSERT INTO roles (org_id, name, is_builtin) VALUES ($1, 'employee', true) RETURNING id`, orgID).Scan(&roleID))
	must(pool.QueryRow(ctx, `INSERT INTO members (org_id, role_id, name, status) VALUES ($1, $2, 'Zhang', 'active') RETURNING id`, orgID, roleID).Scan(&memberID))

	var keyID string
	must(pool.QueryRow(ctx, `
		INSERT INTO virtual_keys (name, secret_hash, secret_prefix, owner_type, owner_member_id, budget_micro_cents)
		VALUES ('k', $1, 'vk-test', 'member', $2, $3) RETURNING id`,
		fmt.Sprintf("hash-%d", time.Now().UnixNano()), memberID, budgetMicroCents).Scan(&keyID))
	return keyID
}

func TestReserveRefusesWhatTheBudgetCannotCover(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 1_000_000) // ¥1

	if _, ok, err := r.ReserveFromVirtualKey(ctx, keyID, 600_000, time.Minute); err != nil || !ok {
		t.Fatalf("first reserve: ok=%v err=%v, want it admitted", ok, err)
	}
	// 600k already promised; 600k more would be ¥1.2 against a ¥1 budget.
	if _, ok, err := r.ReserveFromVirtualKey(ctx, keyID, 600_000, time.Minute); err != nil || ok {
		t.Fatalf("second reserve: ok=%v err=%v, want it refused", ok, err)
	}
}

// The behaviour the old post-hoc deduction could not provide: N calls
// arriving at once must not each be told there is room for the same
// money.
func TestConcurrentReservesCannotCollectivelyOverrunTheBudget(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()

	const budget = 10_000_000 // ¥10
	const each = 1_000_000    // ¥1 -- exactly 10 should fit
	keyID := seedKey(t, pool, budget)

	var wg sync.WaitGroup
	admitted := make([]bool, 50)
	for i := range admitted {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, ok, err := r.ReserveFromVirtualKey(ctx, keyID, each, time.Minute)
			if err != nil {
				t.Errorf("reserve %d: %v", i, err)
				return
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
	if count != budget/each {
		t.Errorf("%d of 50 concurrent calls admitted, want exactly %d", count, budget/each)
	}

	var reserved int64
	if err := pool.QueryRow(ctx, `SELECT reserved_micro_cents FROM virtual_keys WHERE id = $1`, keyID).Scan(&reserved); err != nil {
		t.Fatalf("read reserved: %v", err)
	}
	if reserved > budget {
		t.Errorf("reserved %d micro-cents against a budget of %d", reserved, budget)
	}
}

func TestSettleChargesTheActualCostAndFreesTheReservation(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 10_000_000)

	// Admitted on a ¥5 worst case, actually cost ¥0.30.
	reservation, ok, err := r.ReserveFromVirtualKey(ctx, keyID, 5_000_000, time.Minute)
	if err != nil || !ok {
		t.Fatalf("reserve: ok=%v err=%v", ok, err)
	}
	if err := r.SettleReservation(ctx, reservation, 300_000); err != nil {
		t.Fatalf("settle: %v", err)
	}

	var spent, reserved int64
	if err := pool.QueryRow(ctx,
		`SELECT spent_micro_cents, reserved_micro_cents FROM virtual_keys WHERE id = $1`, keyID,
	).Scan(&spent, &reserved); err != nil {
		t.Fatalf("read key: %v", err)
	}
	if spent != 300_000 {
		t.Errorf("spent = %d, want the actual 300000, not the reserved 5000000", spent)
	}
	if reserved != 0 {
		t.Errorf("reserved = %d, want the hold released", reserved)
	}
}

func TestReleaseGivesTheBudgetBackWithoutCharging(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 10_000_000)

	reservation, _, err := r.ReserveFromVirtualKey(ctx, keyID, 5_000_000, time.Minute)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := r.SettleReservation(ctx, reservation, 0); err != nil {
		t.Fatalf("release: %v", err)
	}

	var spent, reserved int64
	if err := pool.QueryRow(ctx,
		`SELECT spent_micro_cents, reserved_micro_cents FROM virtual_keys WHERE id = $1`, keyID,
	).Scan(&spent, &reserved); err != nil {
		t.Fatalf("read key: %v", err)
	}
	if spent != 0 || reserved != 0 {
		t.Errorf("spent=%d reserved=%d, want both back to zero", spent, reserved)
	}
}

// Real usage can land above what the call was admitted on. Spend records
// what happened rather than clamping, which is what makes the next
// reservation refuse instead of quietly forgiving the overrun.
func TestSettleRecordsAnOverrunRatherThanClampingIt(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 1_000_000) // ¥1

	reservation, _, err := r.ReserveFromVirtualKey(ctx, keyID, 500_000, time.Minute)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := r.SettleReservation(ctx, reservation, 1_500_000); err != nil { // ¥1.50 actual
		t.Fatalf("settle: %v", err)
	}

	var spent int64
	if err := pool.QueryRow(ctx, `SELECT spent_micro_cents FROM virtual_keys WHERE id = $1`, keyID).Scan(&spent); err != nil {
		t.Fatalf("read key: %v", err)
	}
	if spent != 1_500_000 {
		t.Errorf("spent = %d, want the real 1500000 even though it is over budget", spent)
	}
	if _, ok, err := r.ReserveFromVirtualKey(ctx, keyID, 1, time.Minute); err != nil || ok {
		t.Errorf("a key that is already over budget admitted another call (ok=%v err=%v)", ok, err)
	}
}

// A process killed mid-call leaves its reservation behind. Without the
// sweeper that budget stays promised to nothing, forever.
func TestExpireStaleReservationsGivesAbandonedBudgetBack(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 10_000_000)

	if _, _, err := r.ReserveFromVirtualKey(ctx, keyID, 4_000_000, -time.Minute); err != nil { // already expired
		t.Fatalf("reserve: %v", err)
	}
	live, _, err := r.ReserveFromVirtualKey(ctx, keyID, 1_000_000, time.Hour)
	if err != nil {
		t.Fatalf("reserve live: %v", err)
	}

	freed, err := r.ExpireStaleReservations(ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if freed != 1 {
		t.Errorf("freed = %d, want exactly the one expired reservation", freed)
	}

	var reserved int64
	if err := pool.QueryRow(ctx, `SELECT reserved_micro_cents FROM virtual_keys WHERE id = $1`, keyID).Scan(&reserved); err != nil {
		t.Fatalf("read key: %v", err)
	}
	if reserved != 1_000_000 {
		t.Errorf("reserved = %d, want only the still-live reservation to remain", reserved)
	}
	if err := r.SettleReservation(ctx, live, 0); err != nil {
		t.Fatalf("settle live: %v", err)
	}
}

func TestReserveRefusesARevokedKey(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 10_000_000)

	if _, err := r.RevokeVirtualKey(ctx, keyID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, ok, err := r.ReserveFromVirtualKey(ctx, keyID, 1_000, time.Minute); err != nil || ok {
		t.Errorf("a revoked key admitted a call (ok=%v err=%v)", ok, err)
	}
}

// seedModel adds a provider and a model under the org the test seeded,
// returning the model id.
func seedModel(t *testing.T, pool *pgxpool.Pool, name, status, providerStatus string) string {
	t.Helper()
	ctx := context.Background()

	var orgID string
	if err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		t.Fatalf("seed model: no org: %v", err)
	}
	var providerID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO providers (org_id, name, kind, status) VALUES ($1, $2, 'openai_compatible', $3)
		RETURNING id`, orgID, "p-"+name, providerStatus).Scan(&providerID); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	var modelID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO models (provider_id, name, model_identifier, status) VALUES ($1, $2, $2, $3)
		RETURNING id`, providerID, name, status).Scan(&modelID); err != nil {
		t.Fatalf("seed model: %v", err)
	}
	return modelID
}

// /v1/models answers "what may this key call", not "what exists". A
// caller should not learn the names of models it would be refused for
// asking about.
func TestListModelsForVirtualKeyHidesWhatTheKeyCannotCall(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 10_000_000)

	inScope := seedModel(t, pool, "allowed", "published", "active")
	seedModel(t, pool, "other-published", "published", "active")
	seedModel(t, pool, "draft", "draft", "active")
	seedModel(t, pool, "on-disabled-provider", "published", "disabled")

	// No scope set: every published model on an active provider.
	all, err := r.ListModelsForVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := map[string]bool{}
	for _, m := range all {
		names[m.Name] = true
	}
	if !names["allowed"] || !names["other-published"] {
		t.Errorf("published models missing: %v", names)
	}
	if names["draft"] {
		t.Error("a draft model was offered to a caller")
	}
	if names["on-disabled-provider"] {
		t.Error("a model on a disabled provider was offered to a caller")
	}

	// With a scope, only what is in it.
	if _, err := pool.Exec(ctx, `UPDATE virtual_keys SET model_scope = $1 WHERE id = $2`,
		[]string{inScope}, keyID); err != nil {
		t.Fatalf("set scope: %v", err)
	}
	scoped, err := r.ListModelsForVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("list scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Name != "allowed" {
		t.Errorf("scoped list = %+v, want only the in-scope model", scoped)
	}
}

// Revoke reports the secret hash so the caller can drop it from the
// authentication cache instead of leaving the key usable until the entry
// lapses.
func TestRevokeReportsTheSecretItRetired(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 1_000_000)

	var stored string
	if err := pool.QueryRow(ctx, `SELECT secret_hash FROM virtual_keys WHERE id = $1`, keyID).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}

	got, err := r.RevokeVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got != stored {
		t.Errorf("revoke returned %q, want the stored hash %q", got, stored)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM virtual_keys WHERE id = $1`, keyID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "revoked" {
		t.Errorf("status = %q", status)
	}
}

// "Make sure this key is dead" is the request. An admin who clicks it
// twice is asking the same question, not a different one -- and the
// first revocation's timestamp is the one the audit trail wants.
func TestRevokingTwiceSucceedsAndKeepsTheFirstTimestamp(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()
	keyID := seedKey(t, pool, 1_000_000)

	first, err := r.RevokeVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	var revokedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT revoked_at FROM virtual_keys WHERE id = $1`, keyID).Scan(&revokedAt); err != nil {
		t.Fatalf("read revoked_at: %v", err)
	}

	second, err := r.RevokeVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	if second != first {
		t.Errorf("second revoke reported a different hash")
	}

	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT revoked_at FROM virtual_keys WHERE id = $1`, keyID).Scan(&after); err != nil {
		t.Fatalf("read revoked_at again: %v", err)
	}
	if !after.Equal(revokedAt) {
		t.Errorf("revoked_at moved from %s to %s", revokedAt, after)
	}
}

func TestRevokingAKeyThatIsNotThereIsNotFound(t *testing.T) {
	_, r := testPool(t)
	_, err := r.RevokeVirtualKey(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// The whole point, end to end: revoke, and the very next authentication
// carrying that secret sees a revoked key rather than a cached active
// one.
func TestRevokeTakesEffectOnTheNextAuthentication(t *testing.T) {
	pool, r := testPool(t)
	ctx := context.Background()

	raw, prefix, err := keyauth.GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	seedKey(t, pool, 1_000_000) // seeds the org/member a key needs

	var memberID string
	if err := pool.QueryRow(ctx, `SELECT id FROM members LIMIT 1`).Scan(&memberID); err != nil {
		t.Fatalf("read member: %v", err)
	}
	var keyID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO virtual_keys (name, secret_hash, secret_prefix, owner_type, owner_member_id, budget_micro_cents)
		VALUES ('live', $1, $2, 'member', $3, 1000000) RETURNING id`,
		keyauth.HashSecret(raw), prefix, memberID).Scan(&keyID); err != nil {
		t.Fatalf("insert key: %v", err)
	}

	cache := keyauth.NewCache(r)
	got, err := cache.Authenticate(ctx, raw)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.Status != types.VirtualKeyStatusActive {
		t.Fatalf("status = %q, want active before revoking", got.Status)
	}

	secretHash, err := r.RevokeVirtualKey(ctx, keyID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	cache.Forget(secretHash)

	after, err := cache.Authenticate(ctx, raw)
	if err != nil {
		t.Fatalf("authenticate after revoke: %v", err)
	}
	if after.Status != types.VirtualKeyStatusRevoked {
		t.Errorf("status = %q, want revoked immediately rather than within a TTL", after.Status)
	}
}
