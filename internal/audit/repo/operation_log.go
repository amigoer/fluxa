package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/audit/types"
)

// OperationLogRepo stores and reads the admin operation audit trail.
type OperationLogRepo interface {
	InsertOperationLog(ctx context.Context, l types.OperationAuditLog) (types.OperationAuditLog, error)
	ListOperationLogs(ctx context.Context, limit int) ([]types.OperationAuditLog, error)
}

func (r *repo) InsertOperationLog(ctx context.Context, l types.OperationAuditLog) (types.OperationAuditLog, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO operation_audit_logs (actor_member_id, action, detail)
		VALUES ($1, $2, $3)
		RETURNING id, occurred_at`,
		l.ActorMemberID, l.Action, l.Detail,
	).Scan(&l.ID, &l.OccurredAt)
	return l, err
}

func (r *repo) ListOperationLogs(ctx context.Context, limit int) ([]types.OperationAuditLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_member_id, action, detail, occurred_at
		FROM operation_audit_logs ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.OperationAuditLog
	for rows.Next() {
		var l types.OperationAuditLog
		if err := rows.Scan(&l.ID, &l.ActorMemberID, &l.Action, &l.Detail, &l.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
