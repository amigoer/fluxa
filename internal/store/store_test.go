package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fluxa.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProviderCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	p := Provider{
		Name:        "openai",
		Kind:        "openai",
		APIKey:      "sk-test",
		BaseURL:     "https://api.openai.com/v1",
		Models:      []string{"gpt-4o", "gpt-4o-mini"},
		Headers:     map[string]string{"X-Env": "dev"},
		Deployments: map[string]string{"gpt-4o": "prod"},
		TimeoutSec:  300,
		Enabled:     true,
	}
	if err := s.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	got, err := s.GetProvider(ctx, "openai")
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.APIKey != "sk-test" || got.BaseURL != p.BaseURL {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if len(got.Models) != 2 || got.Models[0] != "gpt-4o" {
		t.Errorf("models mismatch: %v", got.Models)
	}
	if got.Headers["X-Env"] != "dev" {
		t.Errorf("headers mismatch: %v", got.Headers)
	}
	if got.Deployments["gpt-4o"] != "prod" {
		t.Errorf("deployments mismatch: %v", got.Deployments)
	}
	if !got.Enabled {
		t.Errorf("enabled lost")
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not populated")
	}

	// update in place
	p.APIKey = "sk-new"
	if err := s.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetProvider(ctx, "openai")
	if got.APIKey != "sk-new" {
		t.Errorf("update not persisted: %s", got.APIKey)
	}

	list, err := s.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 row, got %d", len(list))
	}

	if err := s.DeleteProvider(ctx, "openai"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if _, err := s.GetProvider(ctx, "openai"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteProvider(ctx, "openai"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete should return ErrNotFound, got %v", err)
	}
}

func TestRouteCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Seed provider first (FK constraint is advisory with PRAGMA off, but
	// exercising the happy path keeps the test realistic).
	if err := s.UpsertProvider(ctx, Provider{Name: "openai", Kind: "openai", APIKey: "k", Enabled: true}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := s.UpsertProvider(ctx, Provider{Name: "deepseek", Kind: "deepseek", APIKey: "k", Enabled: true}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	r := Route{Model: "gpt-4o", Provider: "openai", Fallback: []string{"deepseek"}}
	if err := s.UpsertRoute(ctx, r); err != nil {
		t.Fatalf("UpsertRoute: %v", err)
	}

	got, err := s.GetRoute(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if got.Provider != "openai" || len(got.Fallback) != 1 || got.Fallback[0] != "deepseek" {
		t.Errorf("round trip mismatch: %+v", got)
	}

	list, err := s.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 route, got %d", len(list))
	}

	if err := s.DeleteRoute(ctx, "gpt-4o"); err != nil {
		t.Fatalf("DeleteRoute: %v", err)
	}
	if _, err := s.GetRoute(ctx, "gpt-4o"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fluxa.db")
	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	ctx := context.Background()
	if err := s1.UpsertProvider(ctx, Provider{Name: "openai", Kind: "openai", APIKey: "k", Enabled: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = s1.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	defer s2.Close()
	list, err := s2.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "openai" {
		t.Errorf("persisted rows lost after reopen: %+v", list)
	}
}
