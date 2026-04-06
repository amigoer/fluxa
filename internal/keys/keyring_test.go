package keys

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

func newRing(t *testing.T) (*Keyring, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fluxa.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return NewKeyring(s), s
}

func TestAuthorize_HappyPath(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-ok", Name: "ok", Enabled: true,
		Models: []string{"gpt-4o"},
	})
	_ = ring.Reload(ctx)

	if _, err := ring.Authorize(ctx, "vk-ok", "gpt-4o", "1.2.3.4:9999"); err != nil {
		t.Errorf("should allow: %v", err)
	}
}

func TestAuthorize_Disabled(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{ID: "vk-off", Name: "off", Enabled: false})
	_ = ring.Reload(ctx)
	if _, err := ring.Authorize(ctx, "vk-off", "gpt-4o", ""); err != ErrKeyDisabled {
		t.Errorf("expected disabled, got %v", err)
	}
}

func TestAuthorize_Expired(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	past := time.Now().Add(-time.Hour)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-exp", Name: "e", Enabled: true, ExpiresAt: &past,
	})
	_ = ring.Reload(ctx)
	if _, err := ring.Authorize(ctx, "vk-exp", "gpt-4o", ""); err != ErrKeyExpired {
		t.Errorf("expected expired, got %v", err)
	}
}

func TestAuthorize_ModelAllowlist(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-allow", Name: "a", Enabled: true,
		Models: []string{"gpt-4o", "gpt-4o-mini"},
	})
	_ = ring.Reload(ctx)
	if _, err := ring.Authorize(ctx, "vk-allow", "claude-3-5-sonnet", ""); err != ErrModelForbidden {
		t.Errorf("expected forbidden, got %v", err)
	}
	if _, err := ring.Authorize(ctx, "vk-allow", "gpt-4o-mini", ""); err != nil {
		t.Errorf("should allow gpt-4o-mini: %v", err)
	}
}

func TestAuthorize_Wildcard(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-any", Name: "a", Enabled: true, Models: []string{"*"},
	})
	_ = ring.Reload(ctx)
	if _, err := ring.Authorize(ctx, "vk-any", "anything", ""); err != nil {
		t.Errorf("wildcard should allow any model: %v", err)
	}
}

func TestAuthorize_IPAllowlist(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-ip", Name: "ip", Enabled: true,
		IPAllowlist: []string{"10.0.0.0/8", "192.168.1.5"},
	})
	_ = ring.Reload(ctx)
	if _, err := ring.Authorize(ctx, "vk-ip", "gpt-4o", "10.1.2.3:9999"); err != nil {
		t.Errorf("CIDR allow failed: %v", err)
	}
	if _, err := ring.Authorize(ctx, "vk-ip", "gpt-4o", "192.168.1.5:1"); err != nil {
		t.Errorf("exact ip allow failed: %v", err)
	}
	if _, err := ring.Authorize(ctx, "vk-ip", "gpt-4o", "8.8.8.8:1"); err != ErrIPForbidden {
		t.Errorf("expected ip forbidden, got %v", err)
	}
}

func TestAuthorize_RateLimit(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-rl", Name: "rl", Enabled: true, RPMLimit: 2,
	})
	_ = ring.Reload(ctx)

	// Two should pass, the third should be rate-limited.
	if _, err := ring.Authorize(ctx, "vk-rl", "gpt-4o", ""); err != nil {
		t.Errorf("1: %v", err)
	}
	if _, err := ring.Authorize(ctx, "vk-rl", "gpt-4o", ""); err != nil {
		t.Errorf("2: %v", err)
	}
	if _, err := ring.Authorize(ctx, "vk-rl", "gpt-4o", ""); err != ErrRateLimited {
		t.Errorf("3: expected rate-limited, got %v", err)
	}
}

func TestAuthorize_TokenBudget(t *testing.T) {
	ctx := context.Background()
	ring, s := newRing(t)
	_ = s.UpsertVirtualKey(ctx, store.VirtualKey{
		ID: "vk-bud", Name: "b", Enabled: true, BudgetTokensDaily: 100,
	})
	_ = ring.Reload(ctx)

	// Under budget — allowed.
	if _, err := ring.Authorize(ctx, "vk-bud", "gpt-4o", ""); err != nil {
		t.Errorf("initial: %v", err)
	}

	// Log usage that exceeds the daily cap.
	_ = s.InsertUsage(ctx, store.UsageRecord{
		VirtualKeyID: "vk-bud", Ts: time.Now(), Model: "gpt-4o", Provider: "openai",
		TotalTokens: 150,
	})
	ring.InvalidateUsage("vk-bud")

	if _, err := ring.Authorize(ctx, "vk-bud", "gpt-4o", ""); err != ErrBudgetExceeded {
		t.Errorf("expected budget exceeded, got %v", err)
	}
}

func TestGenerateID(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(id) != len(VirtualKeyPrefix)+32 {
		t.Errorf("unexpected length: %d", len(id))
	}
	if !IsVirtualKey(id) {
		t.Errorf("%q should be virtual key", id)
	}
}

func TestExtractBearer(t *testing.T) {
	tok, ok := ExtractBearer("Bearer vk-abc")
	if !ok || tok != "vk-abc" {
		t.Errorf("extract: %q %v", tok, ok)
	}
	if _, ok := ExtractBearer("abc"); ok {
		t.Error("should not parse")
	}
}
