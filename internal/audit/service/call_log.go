package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/audit/types"
)

// CallLogService records and reads the gateway's per-request call log:
// the whole-deployment view for admins, and each member's own view.
type CallLogService interface {
	LogCall(ctx context.Context, l types.CallLog) error
	ListCallLogs(ctx context.Context, limit int) ([]types.CallLog, error)
	ListMyCallLogs(ctx context.Context, memberID string, limit int) ([]types.CallLog, error)
}

func (s *service) LogCall(ctx context.Context, l types.CallLog) error {
	_, err := s.repo.InsertCallLog(ctx, l)
	return err
}

func (s *service) ListCallLogs(ctx context.Context, limit int) ([]types.CallLog, error) {
	return s.repo.ListCallLogs(ctx, limit)
}

func (s *service) ListMyCallLogs(ctx context.Context, memberID string, limit int) ([]types.CallLog, error) {
	return s.repo.ListCallLogsByMember(ctx, memberID, limit)
}
