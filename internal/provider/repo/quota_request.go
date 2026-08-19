package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// QuotaRequestRepo stores the request-and-approve path to more budget:
// the requests themselves, the queues that show them, and who is allowed
// to decide one.
type QuotaRequestRepo interface {
	CreateQuotaRequest(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error)
	GetQuotaRequest(ctx context.Context, id string) (types.QuotaRequest, error)
	ListQuotaRequestsByMember(ctx context.Context, memberID string) ([]types.QuotaRequest, error)
	ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error)
	ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error)
	CanApproveQuotaRequest(ctx context.Context, requestID, deciderMemberID string) (bool, error)
	DecideQuotaRequest(ctx context.Context, id string, status types.QuotaRequestStatus, decidedByMemberID string) error
}

func (r *repo) CreateQuotaRequest(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO quota_requests (requested_by_member_id, model_id, amount_micro_cents, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`,
		q.RequestedByMemberID, q.ModelID, q.AmountMicroCents, q.Reason,
	).Scan(&q.ID, &q.Status, &q.CreatedAt)
	return q, err
}

func (r *repo) GetQuotaRequest(ctx context.Context, id string) (types.QuotaRequest, error) {
	var q types.QuotaRequest
	err := r.pool.QueryRow(ctx, `
		SELECT id, requested_by_member_id, model_id, amount_micro_cents, reason, status, decided_by_member_id, decided_at, created_at
		FROM quota_requests WHERE id = $1`, id,
	).Scan(&q.ID, &q.RequestedByMemberID, &q.ModelID, &q.AmountMicroCents, &q.Reason, &q.Status, &q.DecidedByMemberID, &q.DecidedAt, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.QuotaRequest{}, ErrNotFound
	}
	return q, err
}

func (r *repo) ListQuotaRequestsByMember(ctx context.Context, memberID string) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `WHERE requested_by_member_id = $1`, memberID)
}

func (r *repo) ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `
		WHERE status = 'pending' AND requested_by_member_id IN (
			SELECT id FROM members WHERE department_id = $1
		)`, departmentID)
}

func (r *repo) ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `WHERE status = 'pending'`)
}

func (r *repo) queryQuotaRequests(ctx context.Context, where string, args ...any) ([]types.QuotaRequest, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, requested_by_member_id, model_id, amount_micro_cents, reason, status, decided_by_member_id, decided_at, created_at
		FROM quota_requests %s ORDER BY created_at DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.QuotaRequest
	for rows.Next() {
		var q types.QuotaRequest
		if err := rows.Scan(&q.ID, &q.RequestedByMemberID, &q.ModelID, &q.AmountMicroCents, &q.Reason, &q.Status, &q.DecidedByMemberID, &q.DecidedAt, &q.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// CanApproveQuotaRequest reports whether deciderMemberID is the lead of
// the department the requester in requestID belongs to. It is a single
// SQL join across members/departments rather than a cross-module Go
// call into the user package: modules share one database, and this
// keeps provider decoupled from user's internals for a check this
// narrow. The admin fallback (quota.approve_any) is checked separately,
// against the caller's rbac.Principal, since that doesn't need a query
// at all.
func (r *repo) CanApproveQuotaRequest(ctx context.Context, requestID, deciderMemberID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM quota_requests qr
			JOIN members m ON m.id = qr.requested_by_member_id
			JOIN departments d ON d.id = m.department_id
			WHERE qr.id = $1 AND d.lead_member_id = $2
		)`, requestID, deciderMemberID,
	).Scan(&ok)
	return ok, err
}

func (r *repo) DecideQuotaRequest(ctx context.Context, id string, status types.QuotaRequestStatus, decidedByMemberID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE quota_requests SET status = $1, decided_by_member_id = $2, decided_at = now()
		WHERE id = $3 AND status = 'pending'`,
		status, decidedByMemberID, id,
	)
	return err
}
