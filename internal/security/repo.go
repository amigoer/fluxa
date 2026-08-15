package security

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateRule(ctx context.Context, rule DLPRule) (DLPRule, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO dlp_rules (name, match_type, pattern, action, priority, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		rule.Name, rule.MatchType, rule.Pattern, rule.Action, rule.Priority, rule.Enabled,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	return rule, err
}

func (r *Repo) ListRules(ctx context.Context) ([]DLPRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, match_type, pattern, action, priority, enabled, created_at, updated_at
		FROM dlp_rules ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DLPRule
	for rows.Next() {
		var rule DLPRule
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
func (r *Repo) ListEnabledRulesByPriority(ctx context.Context) ([]DLPRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, match_type, pattern, action, priority, enabled, created_at, updated_at
		FROM dlp_rules WHERE enabled = true ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DLPRule
	for rows.Next() {
		var rule DLPRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MatchType, &rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateRuleEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE dlp_rules SET enabled = $1, updated_at = now() WHERE id = $2`, enabled, id)
	return err
}

func (r *Repo) DeleteRule(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM dlp_rules WHERE id = $1`, id)
	return err
}

func (r *Repo) LogEvent(ctx context.Context, e SecurityEvent) (SecurityEvent, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO security_events (member_id, virtual_key_id, rule_id, description, action_taken)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, occurred_at`,
		e.MemberID, e.VirtualKeyID, e.RuleID, e.Description, e.ActionTaken,
	).Scan(&e.ID, &e.OccurredAt)
	return e, err
}

func (r *Repo) ListEvents(ctx context.Context, limit int) ([]SecurityEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, member_id, virtual_key_id, rule_id, description, action_taken, occurred_at
		FROM security_events ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SecurityEvent
	for rows.Next() {
		var e SecurityEvent
		if err := rows.Scan(&e.ID, &e.MemberID, &e.VirtualKeyID, &e.RuleID, &e.Description, &e.ActionTaken, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
