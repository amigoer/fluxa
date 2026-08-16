package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// RoutingRepo stores the global and personal routing chains.
type RoutingRepo interface {
	CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error)
	ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error)
}

func (r *repo) CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO routing_rules (scope, owner_member_id, condition_label, target_model_id, fallback_model_id, cost_ceiling_cents, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		rule.Scope, rule.OwnerMemberID, rule.ConditionLabel, rule.TargetModelID, rule.FallbackModelID, rule.CostCeilingCents, rule.SortOrder,
	).Scan(&rule.ID, &rule.CreatedAt)
	return rule, err
}

// ListRoutingChain returns the rules for a scope (global, or personal
// for a specific member) in chain order, for routing.Resolver to walk.
func (r *repo) ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error) {
	query := `
		SELECT id, scope, owner_member_id, condition_label, target_model_id, fallback_model_id, cost_ceiling_cents, sort_order, created_at
		FROM routing_rules WHERE scope = $1`
	args := []any{scope}
	if ownerMemberID != nil {
		query += ` AND owner_member_id = $2`
		args = append(args, *ownerMemberID)
	} else {
		query += ` AND owner_member_id IS NULL`
	}
	query += ` ORDER BY sort_order`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.RoutingRule
	for rows.Next() {
		var rule types.RoutingRule
		if err := rows.Scan(&rule.ID, &rule.Scope, &rule.OwnerMemberID, &rule.ConditionLabel, &rule.TargetModelID, &rule.FallbackModelID, &rule.CostCeilingCents, &rule.SortOrder, &rule.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}
