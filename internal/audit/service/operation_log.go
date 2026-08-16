package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/audit/types"
)

// OperationLogService records and reads the admin operation audit trail
// (distinct from call logs -- DESIGN.md v2 roadmap note). Writes come
// almost entirely from MutationRecorder rather than from handlers.
type OperationLogService interface {
	LogOperation(ctx context.Context, actorMemberID, action, detail string) error
	ListOperationLogs(ctx context.Context, limit int) ([]types.OperationAuditLog, error)
}

func (s *service) LogOperation(ctx context.Context, actorMemberID, action, detail string) error {
	_, err := s.repo.InsertOperationLog(ctx, types.OperationAuditLog{
		ActorMemberID: actorMemberID,
		Action:        action,
		Detail:        detail,
	})
	return err
}

func (s *service) ListOperationLogs(ctx context.Context, limit int) ([]types.OperationAuditLog, error) {
	return s.repo.ListOperationLogs(ctx, limit)
}
