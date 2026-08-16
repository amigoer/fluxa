package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/security/types"
)

// EventRepo stores what the 安全事件 page reads: one row per rule that
// fired, never the matched value itself.
type EventRepo interface {
	LogEvent(ctx context.Context, e types.SecurityEvent) (types.SecurityEvent, error)
	ListEvents(ctx context.Context, limit int) ([]types.SecurityEvent, error)
}

func (r *repo) LogEvent(ctx context.Context, e types.SecurityEvent) (types.SecurityEvent, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO security_events (member_id, virtual_key_id, rule_id, description, action_taken)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, occurred_at`,
		e.MemberID, e.VirtualKeyID, e.RuleID, e.Description, e.ActionTaken,
	).Scan(&e.ID, &e.OccurredAt)
	return e, err
}

func (r *repo) ListEvents(ctx context.Context, limit int) ([]types.SecurityEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, member_id, virtual_key_id, rule_id, description, action_taken, occurred_at
		FROM security_events ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.SecurityEvent
	for rows.Next() {
		var e types.SecurityEvent
		if err := rows.Scan(&e.ID, &e.MemberID, &e.VirtualKeyID, &e.RuleID, &e.Description, &e.ActionTaken, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
