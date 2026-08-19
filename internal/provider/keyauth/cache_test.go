package keyauth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// countingStore records how often the cache actually went to the
// database, which is the whole point of it existing.
type countingStore struct {
	mu     sync.Mutex
	hits   int
	key    types.VirtualKey
	notYet bool
}

var errNoSuchKey = errors.New("no such key")

func (s *countingStore) FindVirtualKeyByHash(_ context.Context, _ string) (types.VirtualKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits++
	if s.notYet {
		return types.VirtualKey{}, errNoSuchKey
	}
	return s.key, nil
}

func (s *countingStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func activeKey() types.VirtualKey {
	return types.VirtualKey{ID: "k1", Status: types.VirtualKeyStatusActive}
}

func TestAuthenticateServesRepeatCallsFromTheCache(t *testing.T) {
	store := &countingStore{key: activeKey()}
	cache := NewCache(store)

	for range 5 {
		if _, err := cache.Authenticate(context.Background(), "vk-secret"); err != nil {
			t.Fatalf("authenticate: %v", err)
		}
	}
	if store.count() != 1 {
		t.Errorf("hit the database %d times for 5 calls, want 1", store.count())
	}
}

// The bug this closes: a revoked key stayed usable until its entry
// lapsed. Everything before the quota reservation still ran, the refusal
// that did come back said "out of quota" rather than "revoked", and the
// endpoints that authenticate without reserving -- listing models --
// were not stopped at all.
func TestForgetSendsTheNextCallBackToTheDatabase(t *testing.T) {
	store := &countingStore{key: activeKey()}
	cache := NewCache(store)

	if _, err := cache.Authenticate(context.Background(), "vk-secret"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	cache.Forget(HashSecret("vk-secret"))

	// The key is revoked in the database by now.
	store.mu.Lock()
	store.key = types.VirtualKey{ID: "k1", Status: types.VirtualKeyStatusRevoked}
	store.mu.Unlock()

	got, err := cache.Authenticate(context.Background(), "vk-secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.Status != types.VirtualKeyStatusRevoked {
		t.Error("the cache kept serving the key after it was forgotten")
	}
	if store.count() != 2 {
		t.Errorf("database hits = %d, want the second call to have gone back", store.count())
	}
}

// Forgetting a secret that was never cached must not disturb the others.
func TestForgetIsHarmlessForAnUncachedSecret(t *testing.T) {
	store := &countingStore{key: activeKey()}
	cache := NewCache(store)

	if _, err := cache.Authenticate(context.Background(), "vk-a"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	cache.Forget(HashSecret("vk-never-seen"))

	if _, err := cache.Authenticate(context.Background(), "vk-a"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if store.count() != 1 {
		t.Errorf("database hits = %d, want the untouched entry still cached", store.count())
	}
}

func TestEntriesLapseAfterTheirTTL(t *testing.T) {
	store := &countingStore{key: activeKey()}
	cache := NewCache(store)
	cache.ttl = 20 * time.Millisecond

	if _, err := cache.Authenticate(context.Background(), "vk-secret"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := cache.Authenticate(context.Background(), "vk-secret"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if store.count() != 2 {
		t.Errorf("database hits = %d, want the lapsed entry refetched", store.count())
	}
}

// Every key ever authenticated used to stay in the map, including the
// rotated and revoked ones nothing would look up again.
func TestCacheSweepsLapsedEntries(t *testing.T) {
	store := &countingStore{key: activeKey()}
	cache := NewCache(store)
	cache.ttl = time.Millisecond

	for i := range sweepThreshold + 100 {
		if _, err := cache.Authenticate(context.Background(), string(rune(i))); err != nil {
			t.Fatalf("authenticate: %v", err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := cache.Authenticate(context.Background(), "trigger-sweep"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()
	if size > sweepThreshold {
		t.Errorf("tracked %d entries after a sweep, want the lapsed ones dropped", size)
	}
}

func TestAuthenticateDoesNotCacheAMiss(t *testing.T) {
	store := &countingStore{notYet: true}
	cache := NewCache(store)

	for range 3 {
		if _, err := cache.Authenticate(context.Background(), "vk-bogus"); !errors.Is(err, errNoSuchKey) {
			t.Fatalf("err = %v, want the store's not-found", err)
		}
	}
	if store.count() != 3 {
		t.Errorf("database hits = %d, want every miss to reach the store", store.count())
	}
}

func TestGenerateSecretIsUniqueAndPrefixed(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		raw, prefix, err := GenerateSecret()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[raw] {
			t.Fatal("GenerateSecret returned a duplicate")
		}
		seen[raw] = true
		if len(prefix) != 8 || raw[:8] != prefix {
			t.Errorf("prefix %q does not match secret %q", prefix, raw)
		}
	}
}
