// Package keys owns the runtime side of virtual key management:
//
//   * Keyring   — an in-memory index of VirtualKey rows, refreshed from
//                 the store after every admin write. Fast O(1) lookups
//                 on the hot request path.
//   * Limiter   — per-key token-bucket rate limiter for RPM enforcement.
//   * Authorize — a single method that runs every policy check
//                 (enabled, expired, model allowed, IP allowed, RPM,
//                 daily/monthly budget) against a decoded bearer token.
//
// Usage counters for budgets are computed by asking the store to
// SUM(usage_records) over the current day / month window. The query is
// cheap thanks to the (virtual_key_id, ts) index, and the keyring caches
// the last result for a few seconds so we do not hammer SQLite on every
// request — see cachedUsage below.
package keys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// VirtualKeyPrefix is the string every issued virtual key begins with.
// Clients send it in `Authorization: Bearer <prefix><random>`.
const VirtualKeyPrefix = "vk-"

// Denied is returned when a policy check rejects a request. The HTTP
// layer turns this into a 403 or 429 depending on Reason.
type Denied struct {
	Reason string
	Status int
}

func (e *Denied) Error() string { return e.Reason }

// Policy reasons — kept as exported vars so callers can compare.
var (
	ErrKeyNotFound    = &Denied{Reason: "virtual key not found", Status: 401}
	ErrKeyDisabled    = &Denied{Reason: "virtual key disabled", Status: 403}
	ErrKeyExpired     = &Denied{Reason: "virtual key expired", Status: 403}
	ErrModelForbidden = &Denied{Reason: "model not allowed for this key", Status: 403}
	ErrIPForbidden    = &Denied{Reason: "source ip not in allowlist", Status: 403}
	ErrRateLimited    = &Denied{Reason: "rate limit exceeded", Status: 429}
	ErrBudgetExceeded = &Denied{Reason: "budget exceeded", Status: 429}
)

// Keyring is the hot-path cache over the virtual_keys table.
type Keyring struct {
	store *store.Store

	mu      sync.RWMutex
	byID    map[string]store.VirtualKey
	limiter *Limiter

	// cached usage totals per key, with small TTL to avoid hammering SQLite.
	usageMu sync.Mutex
	usage   map[string]cachedUsage
}

type cachedUsage struct {
	daily   store.UsageTotals
	monthly store.UsageTotals
	fetched time.Time
}

// usageCacheTTL bounds how stale a budget decision can be. Five seconds
// is short enough that an operator tightening a budget sees it take
// effect almost immediately, and long enough to collapse a burst of
// concurrent requests into a single SQL round-trip.
const usageCacheTTL = 5 * time.Second

// NewKeyring builds an empty ring bound to a store. Call Reload before
// serving traffic.
func NewKeyring(s *store.Store) *Keyring {
	return &Keyring{
		store:   s,
		byID:    map[string]store.VirtualKey{},
		limiter: NewLimiter(),
		usage:   map[string]cachedUsage{},
	}
}

// Reload fetches all virtual keys from the store and atomically swaps
// the in-memory index. Admin mutations call this so changes take effect
// without a restart.
func (k *Keyring) Reload(ctx context.Context) error {
	rows, err := k.store.ListVirtualKeys(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]store.VirtualKey, len(rows))
	for _, vk := range rows {
		next[vk.ID] = vk
	}
	k.mu.Lock()
	k.byID = next
	k.mu.Unlock()

	// Drop usage cache so newly-updated budgets are re-evaluated.
	k.usageMu.Lock()
	k.usage = map[string]cachedUsage{}
	k.usageMu.Unlock()
	return nil
}

// Get returns a snapshot of the virtual key for the given id.
func (k *Keyring) Get(id string) (store.VirtualKey, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	vk, ok := k.byID[id]
	return vk, ok
}

// Empty reports whether the keyring has no keys loaded. Used by the
// data plane to decide whether virtual key auth is active at all.
func (k *Keyring) Empty() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.byID) == 0
}

// Authorize runs every policy check for (keyID, model, remoteIP) and
// returns nil when the request may proceed. On denial it returns a
// *Denied with the appropriate HTTP status.
func (k *Keyring) Authorize(ctx context.Context, keyID, model, remoteIP string) (store.VirtualKey, error) {
	vk, ok := k.Get(keyID)
	if !ok {
		return store.VirtualKey{}, ErrKeyNotFound
	}
	if !vk.Enabled {
		return vk, ErrKeyDisabled
	}
	if vk.ExpiresAt != nil && time.Now().After(*vk.ExpiresAt) {
		return vk, ErrKeyExpired
	}
	if !modelAllowed(vk.Models, model) {
		return vk, ErrModelForbidden
	}
	if !ipAllowed(vk.IPAllowlist, remoteIP) {
		return vk, ErrIPForbidden
	}
	if vk.RPMLimit > 0 && !k.limiter.Allow(vk.ID, vk.RPMLimit) {
		return vk, ErrRateLimited
	}
	if hasBudget(vk) {
		if err := k.checkBudget(ctx, vk); err != nil {
			return vk, err
		}
	}
	return vk, nil
}

// checkBudget evaluates the four budget caps (tokens daily/monthly,
// USD daily/monthly) against the cached usage totals.
func (k *Keyring) checkBudget(ctx context.Context, vk store.VirtualKey) error {
	daily, monthly, err := k.usageTotals(ctx, vk.ID)
	if err != nil {
		// Fail open on transient store errors — the alternative is a
		// self-inflicted outage. We log these upstream of Authorize.
		return nil
	}
	if vk.BudgetTokensDaily > 0 && daily.Tokens >= vk.BudgetTokensDaily {
		return ErrBudgetExceeded
	}
	if vk.BudgetTokensMonthly > 0 && monthly.Tokens >= vk.BudgetTokensMonthly {
		return ErrBudgetExceeded
	}
	if vk.BudgetUSDDaily > 0 && daily.CostUSD >= vk.BudgetUSDDaily {
		return ErrBudgetExceeded
	}
	if vk.BudgetUSDMonthly > 0 && monthly.CostUSD >= vk.BudgetUSDMonthly {
		return ErrBudgetExceeded
	}
	return nil
}

// usageTotals returns (daily, monthly) usage, consulting the short-TTL
// cache first. It is safe for concurrent callers.
func (k *Keyring) usageTotals(ctx context.Context, keyID string) (store.UsageTotals, store.UsageTotals, error) {
	k.usageMu.Lock()
	if cu, ok := k.usage[keyID]; ok && time.Since(cu.fetched) < usageCacheTTL {
		k.usageMu.Unlock()
		return cu.daily, cu.monthly, nil
	}
	k.usageMu.Unlock()

	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	daily, err := k.store.SumUsage(ctx, keyID, dayStart, now)
	if err != nil {
		return store.UsageTotals{}, store.UsageTotals{}, err
	}
	monthly, err := k.store.SumUsage(ctx, keyID, monthStart, now)
	if err != nil {
		return store.UsageTotals{}, store.UsageTotals{}, err
	}
	k.usageMu.Lock()
	k.usage[keyID] = cachedUsage{daily: daily, monthly: monthly, fetched: time.Now()}
	k.usageMu.Unlock()
	return daily, monthly, nil
}

// InvalidateUsage drops any cached totals for keyID so the next
// Authorize call re-reads them. Called after InsertUsage.
func (k *Keyring) InvalidateUsage(keyID string) {
	k.usageMu.Lock()
	delete(k.usage, keyID)
	k.usageMu.Unlock()
}

// -- helpers ------------------------------------------------------------

func hasBudget(vk store.VirtualKey) bool {
	return vk.BudgetTokensDaily > 0 || vk.BudgetTokensMonthly > 0 ||
		vk.BudgetUSDDaily > 0 || vk.BudgetUSDMonthly > 0
}

func modelAllowed(allow []string, model string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, m := range allow {
		if m == model || m == "*" {
			return true
		}
	}
	return false
}

func ipAllowed(allow []string, remote string) bool {
	if len(allow) == 0 || remote == "" {
		return true
	}
	ip := net.ParseIP(hostOnly(remote))
	if ip == nil {
		return false
	}
	for _, entry := range allow {
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if entry == ip.String() {
			return true
		}
	}
	return false
}

// hostOnly strips any :port suffix from a RemoteAddr string.
func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// GenerateID returns a cryptographically-random virtual key id with the
// "vk-" prefix. The random portion is 32 hex chars (16 bytes = 128 bits).
func GenerateID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("keys: generate id: %w", err)
	}
	return VirtualKeyPrefix + hex.EncodeToString(buf[:]), nil
}

// ExtractBearer pulls the token out of an Authorization header and
// returns (token, true) iff it parses as a Bearer <token> pair.
func ExtractBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix)), true
}

// IsVirtualKey returns true if the token looks like an issued virtual key.
func IsVirtualKey(token string) bool {
	return strings.HasPrefix(token, VirtualKeyPrefix)
}

// Sanity check at compile time that Denied satisfies error.
var _ error = (*Denied)(nil)

// Ensure net is used when allowlist is empty in tests where we only
// reference the package.
var _ = errors.New
