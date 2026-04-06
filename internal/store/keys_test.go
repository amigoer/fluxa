package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestVirtualKeyCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	expires := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	vk := VirtualKey{
		ID:                  "vk-test-001",
		Name:                "frontend-team",
		Description:         "frontend integration key",
		Models:              []string{"gpt-4o", "gpt-4o-mini"},
		IPAllowlist:         []string{"10.0.0.0/8"},
		BudgetTokensDaily:   100_000,
		BudgetTokensMonthly: 3_000_000,
		BudgetUSDDaily:      10,
		BudgetUSDMonthly:    250,
		RPMLimit:            60,
		Enabled:             true,
		ExpiresAt:           &expires,
	}
	if err := s.UpsertVirtualKey(ctx, vk); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.GetVirtualKey(ctx, vk.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != vk.Name || len(got.Models) != 2 || got.RPMLimit != 60 {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(expires) {
		t.Errorf("expires mismatch: %v", got.ExpiresAt)
	}

	// Update — bump RPM.
	vk.RPMLimit = 120
	if err := s.UpsertVirtualKey(ctx, vk); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetVirtualKey(ctx, vk.ID)
	if got.RPMLimit != 120 {
		t.Errorf("update not persisted")
	}

	list, err := s.ListVirtualKeys(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 row, got %d", len(list))
	}

	if err := s.DeleteVirtualKey(ctx, vk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetVirtualKey(ctx, vk.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUsageInsertAndSum(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.UpsertVirtualKey(ctx, VirtualKey{
		ID: "vk-usage", Name: "u", Enabled: true,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	now := time.Now().UTC()
	records := []UsageRecord{
		{VirtualKeyID: "vk-usage", Ts: now.Add(-2 * time.Hour), Model: "gpt-4o", Provider: "openai",
			PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300, CostUSD: 0.003, LatencyMs: 400, Status: 200},
		{VirtualKeyID: "vk-usage", Ts: now.Add(-1 * time.Hour), Model: "gpt-4o", Provider: "openai",
			PromptTokens: 50, CompletionTokens: 150, TotalTokens: 200, CostUSD: 0.002, LatencyMs: 500, Status: 200},
		{VirtualKeyID: "vk-usage", Ts: now.Add(-30 * time.Minute), Model: "gpt-4o-mini", Provider: "openai",
			PromptTokens: 20, CompletionTokens: 40, TotalTokens: 60, CostUSD: 0.0001, LatencyMs: 120, Status: 200},
	}
	for _, r := range records {
		if err := s.InsertUsage(ctx, r); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	tot, err := s.SumUsage(ctx, "vk-usage", now.Add(-24*time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if tot.Requests != 3 {
		t.Errorf("requests = %d", tot.Requests)
	}
	if tot.Tokens != 560 {
		t.Errorf("tokens = %d", tot.Tokens)
	}
	if tot.CostUSD < 0.005 || tot.CostUSD > 0.006 {
		t.Errorf("cost = %f", tot.CostUSD)
	}

	recent, err := s.RecentUsage(ctx, "vk-usage", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 3 || recent[0].Model != "gpt-4o-mini" {
		t.Errorf("recent order wrong: %+v", recent)
	}
}

func TestDeleteVirtualKeyCascadesUsage(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_ = s.UpsertVirtualKey(ctx, VirtualKey{ID: "vk-drop", Name: "drop", Enabled: true})
	_ = s.InsertUsage(ctx, UsageRecord{VirtualKeyID: "vk-drop", Ts: time.Now(), Model: "m", Provider: "p", TotalTokens: 10})
	_ = s.DeleteVirtualKey(ctx, "vk-drop")

	recent, err := s.RecentUsage(ctx, "vk-drop", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("usage should cascade, got %d rows", len(recent))
	}
}
