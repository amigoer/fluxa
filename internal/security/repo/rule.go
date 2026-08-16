package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/security/types"
)

// RuleRepo stores the DLP rule set: the full list the admin page edits,
// and the enabled subset the scanner walks on every request.
type RuleRepo interface {
	CreateRule(ctx context.Context, rule types.DLPRule) (types.DLPRule, error)
	ListRules(ctx context.Context) ([]types.DLPRule, error)
	ListEnabledRulesByPriority(ctx context.Context) ([]types.DLPRule, error)
	UpdateRuleEnabled(ctx context.Context, id string, enabled bool) error
	DeleteRule(ctx context.Context, id string) error
}

func (r *repo) CreateRule(ctx context.Context, rule types.DLPRule) (types.DLPRule, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO dlp_rules (name, match_type, pattern, action, priority, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		rule.Name, rule.MatchType, rule.Pattern, rule.Action, rule.Priority, rule.Enabled,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	return rule, err
}

func (r *repo) ListRules(ctx context.Context) ([]types.DLPRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, match_type, pattern, action, priority, enabled, created_at, updated_at
		FROM dlp_rules ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.DLPRule
	for rows.Next() {
		var rule types.DLPRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MatchType, &rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// ListEnabledRulesByPriority returns only enabled rules, in the order
// Service.Scan should apply them: lower priority number first, matching
// the mockup's ordering (身份证号识别=10, 银行卡号识别=20, 高风险关键词=5).
func (r *repo) ListEnabledRulesByPriority(ctx context.Context) ([]types.DLPRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, match_type, pattern, action, priority, enabled, created_at, updated_at
		FROM dlp_rules WHERE enabled = true ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.DLPRule
	for rows.Next() {
		var rule types.DLPRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MatchType, &rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *repo) UpdateRuleEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE dlp_rules SET enabled = $1, updated_at = now() WHERE id = $2`, enabled, id)
	return err
}

func (r *repo) DeleteRule(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM dlp_rules WHERE id = $1`, id)
	return err
}
