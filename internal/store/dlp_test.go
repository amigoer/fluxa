package store

import (
	"context"
	"errors"
	"testing"
)

func TestDLPRuleCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	a, err := s.CreateDLPRule(ctx, DLPRule{
		Name:        "block-ssn",
		Pattern:     `\d{3}-\d{2}-\d{4}`,
		PatternType: "regex",
		Scope:       "request",
		Action:      "block",
		Priority:    10,
		Description: "Block social security numbers",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateDLPRule: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected id to be assigned")
	}

	b, err := s.CreateDLPRule(ctx, DLPRule{
		Name:        "log-password",
		Pattern:     "password",
		PatternType: "keyword",
		Scope:       "both",
		Action:      "log",
		Priority:    50,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateDLPRule b: %v", err)
	}

	all, err := s.ListDLPRules(ctx)
	if err != nil {
		t.Fatalf("ListDLPRules: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(all))
	}
	// priority ASC: a (10) before b (50)
	if all[0].ID != a.ID || all[1].ID != b.ID {
		t.Errorf("rows not sorted by priority asc")
	}

	// Update priority.
	if err := s.UpdateDLPRulePriority(ctx, b.ID, 5); err != nil {
		t.Fatalf("UpdateDLPRulePriority: %v", err)
	}
	all, _ = s.ListDLPRules(ctx)
	if all[0].ID != b.ID {
		t.Errorf("after priority bump, b should be first")
	}

	// Full update.
	a.Description = "edited"
	a.Action = "mask"
	if _, err := s.UpdateDLPRule(ctx, a); err != nil {
		t.Fatalf("UpdateDLPRule: %v", err)
	}
	got, _ := s.GetDLPRule(ctx, a.ID)
	if got.Description != "edited" || got.Action != "mask" {
		t.Errorf("update did not persist: %+v", got)
	}

	// Delete.
	if err := s.DeleteDLPRule(ctx, a.ID); err != nil {
		t.Fatalf("DeleteDLPRule: %v", err)
	}
	if _, err := s.GetDLPRule(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDLPRuleValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	cases := []struct {
		name string
		r    DLPRule
	}{
		{"empty name", DLPRule{Pattern: "x", PatternType: "keyword", Scope: "request", Action: "block"}},
		{"empty pattern", DLPRule{Name: "x", PatternType: "keyword", Scope: "request", Action: "block"}},
		{"bad regex", DLPRule{Name: "x", Pattern: "[bad", PatternType: "regex", Scope: "request", Action: "block"}},
		{"bogus pattern_type", DLPRule{Name: "x", Pattern: "x", PatternType: "bogus", Scope: "request", Action: "block"}},
		{"bogus scope", DLPRule{Name: "x", Pattern: "x", PatternType: "keyword", Scope: "bogus", Action: "block"}},
		{"bogus action", DLPRule{Name: "x", Pattern: "x", PatternType: "keyword", Scope: "request", Action: "bogus"}},
		{"bad model_pattern", DLPRule{Name: "x", Pattern: "x", PatternType: "keyword", Scope: "request", Action: "block", ModelPattern: "[bad"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.CreateDLPRule(ctx, tc.r); err == nil {
				t.Errorf("expected validation error, got nil")
			}
		})
	}
}

func TestDLPViolations(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Insert a few violations.
	for i := 0; i < 3; i++ {
		if err := s.InsertDLPViolation(ctx, DLPViolation{
			RuleID:      "rule-1",
			RuleName:    "block-ssn",
			KeyID:       "vk-test",
			Model:       "gpt-4",
			Direction:   "request",
			MatchedText: "123-45-6789",
			ActionTaken: "block",
		}); err != nil {
			t.Fatalf("InsertDLPViolation: %v", err)
		}
	}
	if err := s.InsertDLPViolation(ctx, DLPViolation{
		RuleID:      "rule-2",
		RuleName:    "log-password",
		Direction:   "response",
		MatchedText: "password",
		ActionTaken: "log",
	}); err != nil {
		t.Fatalf("InsertDLPViolation: %v", err)
	}

	// List all.
	vs, total, err := s.ListDLPViolations(ctx, 10, 0, "")
	if err != nil {
		t.Fatalf("ListDLPViolations: %v", err)
	}
	if total != 4 || len(vs) != 4 {
		t.Fatalf("expected 4 violations, got total=%d len=%d", total, len(vs))
	}

	// Filter by rule.
	vs, total, err = s.ListDLPViolations(ctx, 10, 0, "rule-1")
	if err != nil {
		t.Fatalf("ListDLPViolations: %v", err)
	}
	if total != 3 || len(vs) != 3 {
		t.Fatalf("expected 3 violations for rule-1, got total=%d len=%d", total, len(vs))
	}

	// Pagination.
	vs, _, err = s.ListDLPViolations(ctx, 2, 0, "")
	if err != nil {
		t.Fatalf("ListDLPViolations page: %v", err)
	}
	if len(vs) != 2 {
		t.Fatalf("expected 2 rows on first page, got %d", len(vs))
	}

	// Truncation: insert a violation with a very long matched text.
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	if err := s.InsertDLPViolation(ctx, DLPViolation{
		RuleID:      "rule-1",
		RuleName:    "test",
		Direction:   "request",
		MatchedText: string(long),
		ActionTaken: "log",
	}); err != nil {
		t.Fatalf("InsertDLPViolation long: %v", err)
	}
	vs, _, _ = s.ListDLPViolations(ctx, 1, 0, "")
	if len(vs[0].MatchedText) > 200 {
		t.Errorf("expected truncation to 200 chars, got %d", len(vs[0].MatchedText))
	}
}
