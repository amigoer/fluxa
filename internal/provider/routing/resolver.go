// Package routing resolves which model (and its fallback) a request
// should go to: a member's own personal rule for the matching
// condition, falling back to the org-wide global rule for that
// condition, falling back to the global default (DESIGN.md 7.2).
package routing

import (
	"context"
	"errors"

	"github.com/amigoer/fluxa/internal/provider/types"
)

var (
	ErrNoRouteAvailable    = errors.New("routing: no healthy route available")
	ErrCostCeilingExceeded = errors.New("routing: this request would exceed the rule's per-call cost ceiling")
)

const defaultConditionLabel = "默认"

type RuleStore interface {
	ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error)
}

type ModelStore interface {
	GetModel(ctx context.Context, id string) (types.Model, error)
}

// HealthChecker is the breaker, as the resolver needs it. CanAttempt is
// not side-effect free: admitting a request to a recovering provider is
// the same act as claiming that provider's single probe slot, which is
// why Resolved carries whether one was claimed.
type HealthChecker interface {
	CanAttempt(ctx context.Context, providerID string) (allowed, probe bool, err error)
}

type Resolved struct {
	Model types.Model

	// ProbeClaimed reports that the breaker let this request through as
	// the single probe of a recovering provider. The caller owes the
	// breaker an outcome for it: either the call is made and its result
	// recorded, or the probe is handed back (Breaker.ReleaseProbe) so
	// the next request can take it.
	ProbeClaimed bool
}

type Resolver struct {
	rules  RuleStore
	models ModelStore
	health HealthChecker
}

func NewResolver(rules RuleStore, models ModelStore, health HealthChecker) *Resolver {
	return &Resolver{rules: rules, models: models, health: health}
}

// Resolve picks the model a request should be sent to for memberID under
// conditionLabel (e.g. "代码类任务", or defaultConditionLabel when the
// caller has no more specific classification).
//
// inputTokens and requestedMaxTokens size the request for the rule's
// optional CostCeilingMicroCents; they don't need to be exact.
// requestedMaxTokens is the caller's own max_tokens, or 0 when it left
// the output length to the provider -- what that costs is worked out
// per candidate model, since the ceiling that applies depends on the
// model's context window.
//
// The ceiling is checked on *every* hop, primary included. It used to
// guard only the fallback, so an employee could point the primary of
// their personal rule at the most expensive model available and the
// ceiling they had set on that same rule never looked at it.
func (r *Resolver) Resolve(ctx context.Context, memberID, conditionLabel string, inputTokens, requestedMaxTokens int) (Resolved, error) {
	rule, err := r.findRule(ctx, memberID, conditionLabel)
	if err != nil {
		return Resolved{}, err
	}

	target, err := r.models.GetModel(ctx, rule.TargetModelID)
	if err != nil {
		return Resolved{}, err
	}

	primaryWithinCeiling := withinCeiling(rule, target, inputTokens, requestedMaxTokens)
	if primaryWithinCeiling {
		if ok, probe, err := r.health.CanAttempt(ctx, target.ProviderID); err == nil && ok {
			return Resolved{Model: target, ProbeClaimed: probe}, nil
		}
	}

	if rule.FallbackModelID == nil {
		if !primaryWithinCeiling {
			return Resolved{}, ErrCostCeilingExceeded
		}
		return Resolved{}, ErrNoRouteAvailable
	}

	fallback, err := r.models.GetModel(ctx, *rule.FallbackModelID)
	if err != nil {
		return Resolved{}, err
	}

	if !withinCeiling(rule, fallback, inputTokens, requestedMaxTokens) {
		return Resolved{}, ErrCostCeilingExceeded
	}

	ok, probe, err := r.health.CanAttempt(ctx, fallback.ProviderID)
	if err != nil || !ok {
		return Resolved{}, ErrNoRouteAvailable
	}

	return Resolved{Model: fallback, ProbeClaimed: probe}, nil
}

// withinCeiling reports whether sending this request to model would stay
// inside the rule's per-call cost ceiling. A rule with no ceiling set
// admits everything.
func withinCeiling(rule types.RoutingRule, model types.Model, inputTokens, requestedMaxTokens int) bool {
	if rule.CostCeilingMicroCents == nil {
		return true
	}
	outputTokens := MaxOutputTokens(model, requestedMaxTokens, inputTokens)
	return EstimateCostMicroCents(model, inputTokens, outputTokens) <= *rule.CostCeilingMicroCents
}

func (r *Resolver) findRule(ctx context.Context, memberID, conditionLabel string) (types.RoutingRule, error) {
	if personal, err := r.rules.ListRoutingChain(ctx, types.RoutingScopePersonal, &memberID); err == nil {
		if rule, ok := matchLabel(personal, conditionLabel); ok {
			return rule, nil
		}
	}

	global, err := r.rules.ListRoutingChain(ctx, types.RoutingScopeGlobal, nil)
	if err != nil {
		return types.RoutingRule{}, err
	}
	if rule, ok := matchLabel(global, conditionLabel); ok {
		return rule, nil
	}
	if rule, ok := matchLabel(global, defaultConditionLabel); ok {
		return rule, nil
	}

	return types.RoutingRule{}, ErrNoRouteAvailable
}

func matchLabel(rules []types.RoutingRule, label string) (types.RoutingRule, bool) {
	for _, rule := range rules {
		if rule.ConditionLabel == label {
			return rule, true
		}
	}
	return types.RoutingRule{}, false
}

// defaultMaxOutputTokens is what a call is costed against when the
// caller set no max_tokens and the model declares no context window.
// It only has to be a defensible worst case: settling afterwards uses
// the real figure, so erring high costs a little headroom on the key,
// while erring low is what lets a budget be overshot.
const defaultMaxOutputTokens = 4096

// MaxOutputTokens is the most a call could produce, which is what its
// cost has to be checked against before it is allowed to run.
//
// A caller that set max_tokens has told us outright. One that did not
// has left it to the provider, and the old code guessed "about as many
// as it was sent" -- a number with nothing behind it, which then fed
// both the cost ceiling and the budget check. The model's context
// window is the real bound in that case.
func MaxOutputTokens(model types.Model, requestedMaxTokens, inputTokens int) int {
	if requestedMaxTokens > 0 {
		return requestedMaxTokens
	}
	if model.ContextWindow > 0 {
		room := model.ContextWindow - inputTokens
		if room < 0 {
			room = 0
		}
		if room < defaultMaxOutputTokens {
			return room
		}
	}
	return defaultMaxOutputTokens
}

// MicroCentsPerCent is the scale every stored amount is kept at. Money
// is integer minor units end to end (DESIGN.md 12); this is what that
// integer counts, four decimal places below a cent.
const MicroCentsPerCent = 10_000

// EstimateCostMicroCents projects a request's cost from a model's
// per-million-token pricing. The token counts are approximate -- good
// enough to check against a cost ceiling before committing to a hop --
// but the arithmetic on them no longer is.
//
// Computing this in whole cents rounded most real traffic to nothing.
// tokens * cents_per_1M / 1_000_000 truncates toward zero, and at
// ordinary chat sizes the entire result was what got truncated: 800
// tokens at 1000 cents per 1M is 0.8 cents, which billed as 0. Every
// few-hundred-token call cost the org nothing on paper, so spend was
// systematically undercounted and the budget ceiling fed from it never
// fired.
func EstimateCostMicroCents(model types.Model, inputTokens, outputTokens int) int64 {
	return costMicroCents(inputTokens, model.InputPriceCentsPer1M) +
		costMicroCents(outputTokens, model.OutputPriceCentsPer1M)
}

// tokensPerPriceUnit is the denominator a model's price is quoted
// against: InputPriceCentsPer1M is cents per this many tokens.
const tokensPerPriceUnit = 1_000_000

// costMicroCents is tokens * price * MicroCentsPerCent / tokensPerPriceUnit
// with the two constants folded into one divisor, so the multiplication
// that has to happen before the division stays well inside int64.
//
// Rounding is to nearest rather than toward zero: the leftover is under
// half a micro-cent either way, but truncation always loses in the same
// direction, and a bias that can only ever undercount spend is the exact
// shape of bug this change exists to remove.
func costMicroCents(tokens int, priceCentsPer1M int64) int64 {
	const divisor = tokensPerPriceUnit / MicroCentsPerCent
	return (int64(tokens)*priceCentsPer1M + divisor/2) / divisor
}
