package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ProcurementRepo stores what the org actually paid a vendor, the input
// side of the cost picture that call logs only show the spend side of.
type ProcurementRepo interface {
	RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error)
	ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error)
}

func (r *repo) RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO procurement_records (provider_id, amount_micro_cents, note, recorded_by_member_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, recorded_at`,
		rec.ProviderID, rec.AmountMicroCents, rec.Note, rec.RecordedByMemberID,
	).Scan(&rec.ID, &rec.RecordedAt)
	return rec, err
}

func (r *repo) ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pr.id, pr.provider_id, pr.amount_micro_cents, pr.note, pr.recorded_by_member_id, pr.recorded_at
		FROM procurement_records pr
		JOIN providers p ON p.id = pr.provider_id
		WHERE p.org_id = $1
		ORDER BY pr.recorded_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.ProcurementRecord
	for rows.Next() {
		var rec types.ProcurementRecord
		if err := rows.Scan(&rec.ID, &rec.ProviderID, &rec.AmountMicroCents, &rec.Note, &rec.RecordedByMemberID, &rec.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
