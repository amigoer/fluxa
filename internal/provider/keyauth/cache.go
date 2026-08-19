// Package keyauth authenticates virtual keys on the gateway's hot path.
// DB only ever stores a hash of the key; this package hashes the
// incoming secret, checks a short-TTL in-memory cache first, and only
// falls through to the database on a miss, so the overwhelming majority
// of proxied requests never touch the database just to authenticate
// (see DESIGN.md 7.2 "鉴权与一致性").
//
// Revocation does not wait for the TTL: Forget drops an entry the moment
// the key is revoked. The TTL is the backstop for what this process did
// not do itself -- a second replica's revocation, or a row changed
// directly in the database -- not the normal path. Leaving revocation to
// it meant a revoked key kept working for up to a window, and the parts
// of the gateway that authenticate without also reserving quota (listing
// models, say) had nothing else standing between them and a key its
// owner had just cancelled.
package keyauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

const defaultTTL = 45 * time.Second

// sweepThreshold is the number of tracked entries past which a store
// does a cleanup pass. Without it the map only ever grows: every key
// that is ever authenticated stays in it, including the rotated and
// revoked ones nothing will look up again.
const sweepThreshold = 4096

type Store interface {
	FindVirtualKeyByHash(ctx context.Context, secretHash string) (types.VirtualKey, error)
}

type cacheEntry struct {
	key       types.VirtualKey
	expiresAt time.Time
}

type Cache struct {
	store Store
	ttl   time.Duration

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func NewCache(store Store) *Cache {
	return &Cache{store: store, ttl: defaultTTL, entries: make(map[string]cacheEntry)}
}

// Authenticate resolves a raw virtual key secret (as sent by a caller in
// an Authorization header) to the VirtualKey it names, or whatever
// not-found error the backing Store returns (provider.ErrNotFound, in
// practice) if it doesn't match one.
func (c *Cache) Authenticate(ctx context.Context, rawSecret string) (types.VirtualKey, error) {
	hash := HashSecret(rawSecret)

	if entry, ok := c.lookup(hash); ok {
		return entry, nil
	}

	key, err := c.store.FindVirtualKeyByHash(ctx, hash)
	if err != nil {
		return types.VirtualKey{}, err
	}

	c.remember(hash, key)
	return key, nil
}

func (c *Cache) lookup(hash string) (types.VirtualKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[hash]
	if !ok || time.Now().After(entry.expiresAt) {
		return types.VirtualKey{}, false
	}
	return entry.key, true
}

func (c *Cache) remember(hash string, key types.VirtualKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if len(c.entries) > sweepThreshold {
		c.sweep(now)
	}
	c.entries[hash] = cacheEntry{key: key, expiresAt: now.Add(c.ttl)}
}

// sweep drops entries whose window has lapsed. Callers hold the lock.
func (c *Cache) sweep(now time.Time) {
	for hash, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, hash)
		}
	}
}

// Forget drops the cached entry for a key secret, so the next call
// carrying it goes back to the database and sees the current row.
//
// This is what makes revocation take effect now rather than within a
// TTL. It is keyed by secret hash because that is what the cache is
// keyed by -- the caller (provider/service) has the hash from the
// revoke, and holding a second index from key id to hash would be a
// second thing to keep in step for no gain.
func (c *Cache) Forget(secretHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, secretHash)
}

// HashSecret hashes a raw virtual key secret the same way it is stored,
// so a lookup by hash can find it.
func HashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateSecret returns a new raw virtual key secret plus its
// non-secret display prefix (e.g. "vk-8f2a"), used both to show a short
// identifier in the UI and to let an admin recognize which key a log
// line refers to without ever storing the full value.
func GenerateSecret() (raw, prefix string, err error) {
	buf := make([]byte, 24)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = "vk-" + base64.RawURLEncoding.EncodeToString(buf)
	prefix = raw[:8]
	return raw, prefix, nil
}
