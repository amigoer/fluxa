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
	ErrCostCeilingExceeded = errors.New("routing: fallback hop would exceed the rule's cost ceiling")
)

const defaultConditionLabel = "默认"

type RuleStore interface {
	ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error)
}

type ModelStore interface {
	GetModel(ctx context.Context, id string) (types.Model, error)
}

type HealthChecker interface {
	CanAttempt(ctx context.Context, providerID string) (bool, error)
}

type Resolved struct {
	Model types.Model
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
// caller has no more specific classification). estimatedInputTokens and
// estimatedOutputTokens are used only to check a fallback hop's cost
// against the rule's optional CostCeilingCents; they don't need to be
// exact.
func (r *Resolver) Resolve(ctx context.Context, memberID, conditionLabel string, estimatedInputTokens, estimatedOutputTokens int) (Resolved, error) {
	rule, err := r.findRule(ctx, memberID, conditionLabel)
	if err != nil {
		return Resolved{}, err
	}

	target, err := r.models.GetModel(ctx, rule.TargetModelID)
	if err != nil {
		return Resolved{}, err
	}

	if ok, err := r.health.CanAttempt(ctx, target.ProviderID); err == nil && ok {
		return Resolved{Model: target}, nil
	}

	if rule.FallbackModelID == nil {
		return Resolved{}, ErrNoRouteAvailable
	}

	fallback, err := r.models.GetModel(ctx, *rule.FallbackModelID)
	if err != nil {
		return Resolved{}, err
	}

	if rule.CostCeilingCents != nil {
		estimated := EstimateCostCents(fallback, estimatedInputTokens, estimatedOutputTokens)
		if estimated > *rule.CostCeilingCents {
			return Resolved{}, ErrCostCeilingExceeded
		}
	}

	if ok, err := r.health.CanAttempt(ctx, fallback.ProviderID); err != nil || !ok {
		return Resolved{}, ErrNoRouteAvailable
	}

	return Resolved{Model: fallback}, nil
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

// EstimateCostCents projects a request's cost from a model's per-million
// token pricing. It is intentionally approximate -- good enough to
// compare against a cost ceiling before committing to a fallback hop,
// not a substitute for the real cost computed from actual usage after
// the call completes.
func EstimateCostCents(model types.Model, inputTokens, outputTokens int) int64 {
	inputCost := int64(inputTokens) * model.InputPriceCentsPer1M / 1_000_000
	outputCost := int64(outputTokens) * model.OutputPriceCentsPer1M / 1_000_000
	return inputCost + outputCost
}
