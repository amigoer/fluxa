package audit

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

func (r *Repo) InsertCallLog(ctx context.Context, l CallLog) (CallLog, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO call_logs (member_id, virtual_key_id, provider_id, model_id, request_id, status, latency_ms, input_tokens, output_tokens, cost_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, occurred_at`,
		nullIfEmpty(l.MemberID), l.VirtualKeyID, nullIfEmpty(l.ProviderID), nullIfEmpty(l.ModelID), l.RequestID, l.Status, l.LatencyMS, l.InputTokens, l.OutputTokens, l.CostCents,
	).Scan(&l.ID, &l.OccurredAt)
	return l, err
}

func (r *Repo) ListCallLogs(ctx context.Context, limit int) ([]CallLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, member_id, virtual_key_id, provider_id, model_id, request_id, status, latency_ms, input_tokens, output_tokens, cost_cents, occurred_at
		FROM call_logs ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CallLog
	for rows.Next() {
		var l CallLog
		var member, provider, model *string
		if err := rows.Scan(&l.ID, &member, &l.VirtualKeyID, &provider, &model, &l.RequestID, &l.Status, &l.LatencyMS, &l.InputTokens, &l.OutputTokens, &l.CostCents, &l.OccurredAt); err != nil {
			return nil, err
		}
		l.MemberID, l.ProviderID, l.ModelID = deref(member), deref(provider), deref(model)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) ListCallLogsByMember(ctx context.Context, memberID string, limit int) ([]CallLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, member_id, virtual_key_id, provider_id, model_id, request_id, status, latency_ms, input_tokens, output_tokens, cost_cents, occurred_at
		FROM call_logs WHERE member_id = $1 ORDER BY occurred_at DESC LIMIT $2`, memberID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CallLog
	for rows.Next() {
		var l CallLog
		var member, provider, model *string
		if err := rows.Scan(&l.ID, &member, &l.VirtualKeyID, &provider, &model, &l.RequestID, &l.Status, &l.LatencyMS, &l.InputTokens, &l.OutputTokens, &l.CostCents, &l.OccurredAt); err != nil {
			return nil, err
		}
		l.MemberID, l.ProviderID, l.ModelID = deref(member), deref(provider), deref(model)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) InsertOperationLog(ctx context.Context, l OperationAuditLog) (OperationAuditLog, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO operation_audit_logs (actor_member_id, action, detail)
		VALUES ($1, $2, $3)
		RETURNING id, occurred_at`,
		l.ActorMemberID, l.Action, l.Detail,
	).Scan(&l.ID, &l.OccurredAt)
	return l, err
}

func (r *Repo) ListOperationLogs(ctx context.Context, limit int) ([]OperationAuditLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_member_id, action, detail, occurred_at
		FROM operation_audit_logs ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OperationAuditLog
	for rows.Next() {
		var l OperationAuditLog
		if err := rows.Scan(&l.ID, &l.ActorMemberID, &l.Action, &l.Detail, &l.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// call_logs allows a null member, provider and model: a department-owned
// key has no member, and a request rejected before routing has neither
// provider nor model. The domain keeps plain strings with "" meaning
// "not applicable", so these two convert at the database boundary and the
// JSON the console consumes is unchanged.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
