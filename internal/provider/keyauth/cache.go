// Package keyauth authenticates virtual keys on the gateway's hot path.
// DB only ever stores a hash of the key; this package hashes the
// incoming secret, checks a short-TTL in-memory cache first, and only
// falls through to the database on a miss. A revoked key can stay valid
// in the cache for up to one TTL window -- an accepted tradeoff so the
// overwhelming majority of proxied requests never touch the database
// just to authenticate (see DESIGN.md 7.2 "鉴权与一致性").
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
	c.entries[hash] = cacheEntry{key: key, expiresAt: time.Now().Add(c.ttl)}
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
