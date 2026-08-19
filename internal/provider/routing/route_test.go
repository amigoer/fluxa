package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

type stubRules struct {
	personal, global []types.RoutingRule
	personalErr      error
}

func (s stubRules) ListRoutingChain(_ context.Context, scope types.RoutingScope, _ *string) ([]types.RoutingRule, error) {
	if scope == types.RoutingScopePersonal {
		return s.personal, s.personalErr
	}
	return s.global, nil
}

type stubModels map[string]types.Model

func (m stubModels) GetModel(_ context.Context, id string) (types.Model, error) {
	model, ok := m[id]
	if !ok {
		return types.Model{}, errors.New("no such model")
	}
	return model, nil
}

// stubHealth maps a provider to its breaker state. A provider listed as
// probing is one whose breaker is recovering: attemptable, but only by
// claiming its single probe slot.
type stubHealth struct {
	open    map[string]bool
	probing map[string]bool
}

func newStubHealth() stubHealth {
	return stubHealth{open: map[string]bool{}, probing: map[string]bool{}}
}

func (h stubHealth) CanAttempt(_ context.Context, providerID string) (bool, bool, error) {
	if h.probing[providerID] {
		return true, true, nil
	}
	return !h.open[providerID], false, nil
}

func ptr[T any](v T) *T { return &v }

const (
	cheapID = "cheap"
	dearID  = "dear"
)

func fixture() (stubModels, stubHealth) {
	return stubModels{
			// ¥1 / 1M in, ¥2 / 1M out
			cheapID: {ID: cheapID, ProviderID: "p1", InputPriceCentsPer1M: 100, OutputPriceCentsPer1M: 200, ContextWindow: 128_000},
			// ¥100 / 1M in, ¥200 / 1M out
			dearID: {ID: dearID, ProviderID: "p2", InputPriceCentsPer1M: 10_000, OutputPriceCentsPer1M: 20_000, ContextWindow: 128_000},
		},
		newStubHealth()
}

func TestResolvePrefersThePersonalRuleOverTheGlobalOne(t *testing.T) {
	models, health := fixture()
	r := NewResolver(stubRules{
		personal: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: dearID}},
		global:   []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: cheapID}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "默认", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Model.ID != dearID {
		t.Errorf("model = %s, want the personal rule's target", got.Model.ID)
	}
}

func TestResolveFallsBackToTheGlobalDefaultForAnUnknownCondition(t *testing.T) {
	models, health := fixture()
	r := NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: cheapID}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "没有这个条件", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Model.ID != cheapID {
		t.Errorf("model = %s, want the global default", got.Model.ID)
	}
}

func TestResolveTakesTheFallbackWhenThePrimaryProviderIsCircuitOpen(t *testing.T) {
	models, health := fixture()
	health.open["p2"] = true // dear model's provider is down

	r := NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: dearID, FallbackModelID: ptr(cheapID)}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "默认", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Model.ID != cheapID {
		t.Errorf("model = %s, want the fallback", got.Model.ID)
	}
}

// The ceiling used to guard only the fallback hop, so pointing a rule's
// primary at the most expensive model available sailed straight past the
// ceiling set on that same rule.
func TestCostCeilingAppliesToThePrimaryHopNotJustTheFallback(t *testing.T) {
	models, health := fixture()
	// 100k in + 4k out on the dear model is ~¥10.8; ceiling is ¥1.
	r := NewResolver(stubRules{
		global: []types.RoutingRule{{
			ConditionLabel:        "默认",
			TargetModelID:         dearID,
			CostCeilingMicroCents: ptr(int64(1_000_000)),
		}},
	}, models, health)

	_, err := r.Resolve(context.Background(), "m1", "默认", 100_000, 0)
	if !errors.Is(err, ErrCostCeilingExceeded) {
		t.Fatalf("err = %v, want ErrCostCeilingExceeded", err)
	}
}

func TestCostCeilingLetsAnAffordablePrimaryThrough(t *testing.T) {
	models, health := fixture()
	r := NewResolver(stubRules{
		global: []types.RoutingRule{{
			ConditionLabel:        "默认",
			TargetModelID:         cheapID,
			CostCeilingMicroCents: ptr(int64(1_000_000)),
		}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "默认", 100_000, 0)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Model.ID != cheapID {
		t.Errorf("model = %s", got.Model.ID)
	}
}

func TestResolveReportsNoRouteWhenEverythingIsDown(t *testing.T) {
	models, health := fixture()
	health.open["p1"], health.open["p2"] = true, true

	r := NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: dearID, FallbackModelID: ptr(cheapID)}},
	}, models, health)

	if _, err := r.Resolve(context.Background(), "m1", "默认", 100, 100); !errors.Is(err, ErrNoRouteAvailable) {
		t.Fatalf("err = %v, want ErrNoRouteAvailable", err)
	}
}

func TestMaxOutputTokens(t *testing.T) {
	small := types.Model{ContextWindow: 8_000}
	none := types.Model{}

	for _, tc := range []struct {
		name                         string
		model                        types.Model
		requested, inputTokens, want int
	}{
		{"the caller capped its own output", small, 512, 100, 512},
		{"uncapped, bounded by the context window", small, 0, 7_000, 1_000},
		{"uncapped, window has plenty of room", small, 0, 100, defaultMaxOutputTokens},
		{"uncapped, input already fills the window", small, 0, 9_000, 0},
		{"uncapped, model declares no window", none, 0, 100, defaultMaxOutputTokens},
	} {
		if got := MaxOutputTokens(tc.model, tc.requested, tc.inputTokens); got != tc.want {
			t.Errorf("%s: MaxOutputTokens = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// Routing has to report when admitting a request cost the provider its
// single probe slot, because whoever gets the answer then owes the
// breaker an outcome for it.
func TestResolveReportsWhenItClaimedAProbe(t *testing.T) {
	models, health := fixture()
	health.probing["p1"] = true

	r := NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: cheapID}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "默认", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !got.ProbeClaimed {
		t.Error("a probe was claimed but Resolve did not say so")
	}

	// A healthy provider costs nothing to attempt.
	healthy := newStubHealth()
	r = NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: cheapID}},
	}, models, healthy)
	got, err = r.Resolve(context.Background(), "m1", "默认", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.ProbeClaimed {
		t.Error("attempting a healthy provider was reported as a probe claim")
	}
}

func TestResolveReportsAProbeClaimedOnTheFallback(t *testing.T) {
	models, health := fixture()
	health.open["p2"] = true    // primary is down
	health.probing["p1"] = true // fallback is recovering

	r := NewResolver(stubRules{
		global: []types.RoutingRule{{ConditionLabel: "默认", TargetModelID: dearID, FallbackModelID: ptr(cheapID)}},
	}, models, health)

	got, err := r.Resolve(context.Background(), "m1", "默认", 100, 100)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Model.ID != cheapID {
		t.Fatalf("model = %s, want the fallback", got.Model.ID)
	}
	if !got.ProbeClaimed {
		t.Error("the fallback's probe claim was not reported")
	}
}
