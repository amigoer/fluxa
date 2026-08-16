package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ProcurementService records what the org paid a vendor, the side of the
// cost picture the console shows next to what calls actually spent.
type ProcurementService interface {
	RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error)
	ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error)
}

func (s *service) RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error) {
	return s.repo.RecordProcurement(ctx, rec)
}

func (s *service) ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error) {
	return s.repo.ListProcurementRecords(ctx, orgID)
}
