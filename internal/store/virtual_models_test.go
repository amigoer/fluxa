package store

import (
	"context"
	"errors"
	"testing"
)

func TestVirtualModelCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	vm := VirtualModel{
		Name:        "qwen-latest",
		Description: "rolling alias",
		Enabled:     true,
		Routes: []VirtualModelRoute{
			{Weight: 30, TargetType: "real", TargetModel: "qwen2.5-72b", Provider: "qwen", Enabled: true},
			{Weight: 70, TargetType: "real", TargetModel: "qwen3-72b", Provider: "qwen", Enabled: true},
		},
	}
	saved, err := s.UpsertVirtualModel(ctx, vm)
	if err != nil {
		t.Fatalf("UpsertVirtualModel: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("expected an id to be assigned")
	}
	if len(saved.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(saved.Routes))
	}
	if saved.Routes[0].Position != 0 || saved.Routes[1].Position != 1 {
		t.Errorf("positions not assigned in input order: %+v", saved.Routes)
	}

	// Re-upsert with a different shape — should fully replace, not append.
	vm2 := saved
	vm2.Routes = []VirtualModelRoute{
		{Weight: 100, TargetType: "real", TargetModel: "qwen3-72b", Provider: "qwen", Enabled: true},
	}
	saved2, err := s.UpsertVirtualModel(ctx, vm2)
	if err != nil {
		t.Fatalf("UpsertVirtualModel replace: %v", err)
	}
	if saved2.ID != saved.ID {
		t.Errorf("id should remain stable across upserts: %q -> %q", saved.ID, saved2.ID)
	}
	if len(saved2.Routes) != 1 {
		t.Errorf("expected route list to be replaced, got %d rows", len(saved2.Routes))
	}

	all, err := s.ListVirtualModels(ctx)
	if err != nil {
		t.Fatalf("ListVirtualModels: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 virtual model, got %d", len(all))
	}

	if err := s.DeleteVirtualModel(ctx, "qwen-latest"); err != nil {
		t.Fatalf("DeleteVirtualModel: %v", err)
	}
	if _, err := s.GetVirtualModel(ctx, "qwen-latest"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestVirtualModelValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	cases := []struct {
		name string
		vm   VirtualModel
	}{
		{
			name: "no routes",
			vm:   VirtualModel{Name: "x"},
		},
		{
			name: "zero weight",
			vm: VirtualModel{Name: "x", Routes: []VirtualModelRoute{
				{Weight: 0, TargetType: "real", TargetModel: "y", Provider: "p"},
			}},
		},
		{
			name: "real without provider",
			vm: VirtualModel{Name: "x", Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "real", TargetModel: "y"},
			}},
		},
		{
			name: "bogus target_type",
			vm: VirtualModel{Name: "x", Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "bogus", TargetModel: "y", Provider: "p"},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.UpsertVirtualModel(ctx, tc.vm); err == nil {
				t.Errorf("expected validation error, got nil")
			}
		})
	}
}
