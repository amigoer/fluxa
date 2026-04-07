package store

import (
	"context"
	"errors"
	"testing"
)

func TestRegexRouteCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	a, err := s.CreateRegexRoute(ctx, RegexRoute{
		Pattern:     "^gpt-4",
		Priority:    50,
		TargetType:  "virtual",
		TargetModel: "qwen-latest",
		Description: "intercept gpt-4*",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateRegexRoute: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected id to be assigned")
	}

	b, err := s.CreateRegexRoute(ctx, RegexRoute{
		Pattern:     "^claude-",
		Priority:    10,
		TargetType:  "real",
		TargetModel: "claude-3-5-sonnet",
		Provider:    "anthropic",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateRegexRoute b: %v", err)
	}

	all, err := s.ListRegexRoutes(ctx)
	if err != nil {
		t.Fatalf("ListRegexRoutes: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(all))
	}
	// priority ASC: b (10) before a (50)
	if all[0].ID != b.ID || all[1].ID != a.ID {
		t.Errorf("rows not sorted by priority asc: %+v", all)
	}

	// Update path: change priority via the dedicated endpoint.
	if err := s.UpdateRegexRoutePriority(ctx, a.ID, 5); err != nil {
		t.Fatalf("UpdateRegexRoutePriority: %v", err)
	}
	all, _ = s.ListRegexRoutes(ctx)
	if all[0].ID != a.ID {
		t.Errorf("after priority bump, a should be first: %+v", all)
	}

	// Full update.
	a.Description = "edited"
	a.Pattern = "^gpt-4-turbo"
	if _, err := s.UpdateRegexRoute(ctx, a); err != nil {
		t.Fatalf("UpdateRegexRoute: %v", err)
	}
	got, _ := s.GetRegexRoute(ctx, a.ID)
	if got.Description != "edited" || got.Pattern != "^gpt-4-turbo" {
		t.Errorf("update did not persist: %+v", got)
	}

	if err := s.DeleteRegexRoute(ctx, a.ID); err != nil {
		t.Fatalf("DeleteRegexRoute: %v", err)
	}
	if _, err := s.GetRegexRoute(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestRegexRouteValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	cases := []struct {
		name string
		r    RegexRoute
	}{
		{"empty pattern", RegexRoute{TargetType: "real", TargetModel: "x", Provider: "p"}},
		{"bad regex", RegexRoute{Pattern: "[unterminated", TargetType: "real", TargetModel: "x", Provider: "p"}},
		{"bogus target_type", RegexRoute{Pattern: "^.+$", TargetType: "bogus", TargetModel: "x"}},
		{"real without provider", RegexRoute{Pattern: "^.+$", TargetType: "real", TargetModel: "x"}},
		{"missing target", RegexRoute{Pattern: "^.+$", TargetType: "virtual"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.CreateRegexRoute(ctx, tc.r); err == nil {
				t.Errorf("expected validation error, got nil")
			}
		})
	}
}
