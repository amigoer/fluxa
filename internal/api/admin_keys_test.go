package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

func TestAdmin_VirtualKeyLifecycle(t *testing.T) {
	mux, st, key := newAdminFixture(t)

	// Create.
	rec := doAdmin(t, mux, "POST", "/admin/keys", key, map[string]any{
		"name":        "dev laptop",
		"description": "primary key for engineer A",
		"models":      []string{"gpt-4o"},
		"rpm_limit":   60,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d body=%s", rec.Code, rec.Body.String())
	}
	var created virtualKeyDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.ID[:3] != "vk-" {
		t.Errorf("expected vk- prefixed id, got %q", created.ID)
	}
	if created.Enabled == nil || !*created.Enabled {
		t.Errorf("expected enabled=true by default")
	}

	// List returns the newly-created row.
	rec = doAdmin(t, mux, "GET", "/admin/keys", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	var list struct {
		Data []virtualKeyDTO `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.ID {
		t.Errorf("list mismatch: %+v", list.Data)
	}

	// Get by id.
	rec = doAdmin(t, mux, "GET", "/admin/keys/"+created.ID, key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status %d", rec.Code)
	}

	// Update: flip enabled and bump the RPM.
	disabled := false
	rec = doAdmin(t, mux, "PUT", "/admin/keys/"+created.ID, key, map[string]any{
		"enabled":   disabled,
		"rpm_limit": 120,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: status %d body=%s", rec.Code, rec.Body.String())
	}
	var updated virtualKeyDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Enabled == nil || *updated.Enabled {
		t.Errorf("enabled should be false after update")
	}
	if updated.RPMLimit != 120 {
		t.Errorf("rpm should be 120, got %d", updated.RPMLimit)
	}
	// Name must survive partial patch because updateKey starts from the existing row.
	if updated.Name != "dev laptop" {
		t.Errorf("name clobbered: %q", updated.Name)
	}

	// Delete.
	rec = doAdmin(t, mux, "DELETE", "/admin/keys/"+created.ID, key, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}
	if _, err := st.GetVirtualKey(t.Context(), created.ID); err == nil {
		t.Errorf("expected not-found after delete")
	}
}

func TestAdmin_UsageEndpoints(t *testing.T) {
	mux, st, key := newAdminFixture(t)

	// Seed a key and a couple of usage rows directly in the store.
	vk := store.VirtualKey{ID: "vk-test", Name: "t", Enabled: true}
	if err := st.UpsertVirtualKey(t.Context(), vk); err != nil {
		t.Fatalf("seed vk: %v", err)
	}
	now := time.Now()
	for i := 0; i < 3; i++ {
		_ = st.InsertUsage(t.Context(), store.UsageRecord{
			VirtualKeyID: "vk-test", Ts: now, Model: "gpt-4o", Provider: "openai",
			PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, CostUSD: 0.01,
		})
	}

	// Recent list.
	rec := doAdmin(t, mux, "GET", "/admin/usage?key_id=vk-test&limit=5", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list usage: %d", rec.Code)
	}
	var list struct {
		Data []store.UsageRecord `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Data) != 3 {
		t.Errorf("expected 3 usage rows, got %d", len(list.Data))
	}

	// Summary for the single key.
	rec = doAdmin(t, mux, "GET", "/admin/usage/summary?key_id=vk-test", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary: %d", rec.Code)
	}
	var sum struct {
		Daily   store.UsageTotals `json:"daily"`
		Monthly store.UsageTotals `json:"monthly"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.Daily.Requests != 3 || sum.Daily.Tokens != 90 {
		t.Errorf("daily totals off: %+v", sum.Daily)
	}
	if sum.Monthly.Requests != 3 {
		t.Errorf("monthly totals off: %+v", sum.Monthly)
	}

	// Summary across all keys (no key_id).
	rec = doAdmin(t, mux, "GET", "/admin/usage/summary", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("global summary: %d", rec.Code)
	}
}
