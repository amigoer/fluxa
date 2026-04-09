package router

import (
	"errors"
	"math/rand"
	"regexp"
	"testing"
)

// newResolverRouter builds a Router with the resolver fields populated
// directly, bypassing the store. Tests want exact control over the
// virtual model and regex model tables, so we hand-build the state
// rather than threading a real *store.Store through.
func newResolverRouter(virtual map[string]*VirtualModel, regex []*CompiledRegexModel) *Router {
	r := New()
	r.mu.Lock()
	r.state.virtualModels = virtual
	r.state.regexModels = regex
	r.mu.Unlock()
	return r
}

func TestResolveModel_Passthrough(t *testing.T) {
	r := newResolverRouter(nil, nil)
	target, trace, err := r.ResolveModel("gpt-4o")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if target != nil {
		t.Errorf("expected passthrough (nil target), got %+v", target)
	}
	if len(trace) != 1 || trace[0].Type != "passthrough" {
		t.Errorf("expected single passthrough trace step, got %+v", trace)
	}
}

func TestResolveModel_VirtualSingleRoute(t *testing.T) {
	vm := map[string]*VirtualModel{
		"qwen-latest": {
			Name: "qwen-latest",
			Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "real", TargetModel: "qwen3-72b", Provider: "qwen"},
			},
		},
	}
	r := newResolverRouter(vm, nil)
	target, trace, err := r.ResolveModel("qwen-latest")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if target == nil || target.Provider != "qwen" || target.Model != "qwen3-72b" {
		t.Errorf("unexpected target: %+v", target)
	}
	if len(trace) != 1 || trace[0].Type != "virtual_model" || trace[0].Target != "qwen3-72b" {
		t.Errorf("unexpected trace: %+v", trace)
	}
}

func TestResolveModel_WeightedDistribution(t *testing.T) {
	// Pin the rng so the test is deterministic. With Source(1) the
	// sequence of Intn(100) values starts at 81, so a 30/70 split
	// hits the second route on the first call.
	SetResolverRand(rand.New(rand.NewSource(1)))
	defer SetResolverRand(rand.New(rand.NewSource(0))) // reset

	vm := map[string]*VirtualModel{
		"split": {
			Name: "split",
			Routes: []VirtualModelRoute{
				{Weight: 30, TargetType: "real", TargetModel: "model-a", Provider: "p"},
				{Weight: 70, TargetType: "real", TargetModel: "model-b", Provider: "p"},
			},
		},
	}
	r := newResolverRouter(vm, nil)

	// Run a few thousand picks and check the empirical distribution
	// is in the right ballpark. We don't pin the seed for this part
	// because we're checking statistical behaviour, not exact output.
	SetResolverRand(rand.New(rand.NewSource(42)))
	const N = 10000
	hits := map[string]int{}
	for i := 0; i < N; i++ {
		target, _, err := r.ResolveModel("split")
		if err != nil {
			t.Fatalf("ResolveModel: %v", err)
		}
		hits[target.Model]++
	}
	// 30/70 split, allow ±5% slack.
	a := float64(hits["model-a"]) / N
	b := float64(hits["model-b"]) / N
	if a < 0.25 || a > 0.35 {
		t.Errorf("model-a empirical share %.3f outside 0.25..0.35", a)
	}
	if b < 0.65 || b > 0.75 {
		t.Errorf("model-b empirical share %.3f outside 0.65..0.75", b)
	}
}

func TestResolveModel_VirtualToVirtual(t *testing.T) {
	vm := map[string]*VirtualModel{
		"outer": {
			Name: "outer",
			Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "virtual", TargetModel: "inner"},
			},
		},
		"inner": {
			Name: "inner",
			Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "real", TargetModel: "real-model", Provider: "p"},
			},
		},
	}
	r := newResolverRouter(vm, nil)
	target, trace, err := r.ResolveModel("outer")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if target == nil || target.Model != "real-model" {
		t.Errorf("expected real-model, got %+v", target)
	}
	if len(trace) != 2 {
		t.Fatalf("expected 2 trace steps, got %d: %+v", len(trace), trace)
	}
	if trace[0].Depth != 0 || trace[1].Depth != 1 {
		t.Errorf("unexpected depth chain: %+v", trace)
	}
}

func TestResolveModel_RegexInterception(t *testing.T) {
	vm := map[string]*VirtualModel{
		"qwen-latest": {
			Name: "qwen-latest",
			Routes: []VirtualModelRoute{
				{Weight: 1, TargetType: "real", TargetModel: "qwen3-72b", Provider: "qwen"},
			},
		},
	}
	regex := []*CompiledRegexModel{
		mustCompileModel(t, "^gpt-4", 50, "virtual", "qwen-latest", ""),
	}
	r := newResolverRouter(vm, regex)

	target, trace, err := r.ResolveModel("gpt-4-turbo")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if target == nil || target.Model != "qwen3-72b" {
		t.Errorf("expected redirect to qwen3-72b, got %+v", target)
	}
	if len(trace) != 2 || trace[0].Type != "regex_match" || trace[1].Type != "virtual_model" {
		t.Errorf("expected regex → virtual chain in trace, got %+v", trace)
	}
}

func TestResolveModel_RegexPriorityOrdering(t *testing.T) {
	// Two patterns that both match "gpt-4-turbo"; the lower-priority
	// number must win even though it was inserted second in the slice.
	regex := []*CompiledRegexModel{
		mustCompileModel(t, "^gpt-4", 50, "real", "fallback", "p"),
		mustCompileModel(t, "^gpt-4-turbo", 10, "real", "winner", "p"),
	}
	// The router sorts in ReloadRegexModels, so simulate that here.
	regex[0], regex[1] = regex[1], regex[0]

	r := newResolverRouter(nil, regex)
	target, _, err := r.ResolveModel("gpt-4-turbo")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if target == nil || target.Model != "winner" {
		t.Errorf("expected priority winner, got %+v", target)
	}
}

func TestResolveModel_DepthCap(t *testing.T) {
	// Cycle: a → b → a → ...
	vm := map[string]*VirtualModel{
		"a": {Name: "a", Routes: []VirtualModelRoute{{Weight: 1, TargetType: "virtual", TargetModel: "b"}}},
		"b": {Name: "b", Routes: []VirtualModelRoute{{Weight: 1, TargetType: "virtual", TargetModel: "a"}}},
	}
	r := newResolverRouter(vm, nil)
	if _, _, err := r.ResolveModel("a"); !errors.Is(err, ErrResolveDepth) {
		t.Errorf("expected ErrResolveDepth, got %v", err)
	}
}

func TestResolveModel_VirtualWithMissingTarget(t *testing.T) {
	vm := map[string]*VirtualModel{
		"x": {Name: "x", Routes: []VirtualModelRoute{
			{Weight: 1, TargetType: "virtual", TargetModel: "does-not-exist"},
		}},
	}
	r := newResolverRouter(vm, nil)
	if _, _, err := r.ResolveModel("x"); err == nil {
		t.Error("expected error for missing virtual target")
	}
}

func mustCompileModel(t *testing.T, pattern string, priority int, ttype, target, provider string) *CompiledRegexModel {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("compile %q: %v", pattern, err)
	}
	return &CompiledRegexModel{
		Pattern:     re,
		PatternRaw:  pattern,
		Priority:    priority,
		TargetType:  ttype,
		TargetModel: target,
		Provider:    provider,
	}
}
