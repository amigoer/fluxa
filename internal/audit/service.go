package audit

import "context"

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) LogCall(ctx context.Context, l CallLog) error {
	_, err := s.repo.InsertCallLog(ctx, l)
	return err
}

func (s *Service) ListCallLogs(ctx context.Context, limit int) ([]CallLog, error) {
	return s.repo.ListCallLogs(ctx, limit)
}

func (s *Service) ListMyCallLogs(ctx context.Context, memberID string, limit int) ([]CallLog, error) {
	return s.repo.ListCallLogsByMember(ctx, memberID, limit)
}

// LogOperation records an admin action for the operation audit trail
// (distinct from call logs -- DESIGN.md v2 roadmap note).
func (s *Service) LogOperation(ctx context.Context, actorMemberID, action, detail string) error {
	_, err := s.repo.InsertOperationLog(ctx, OperationAuditLog{
		ActorMemberID: actorMemberID,
		Action:        action,
		Detail:        detail,
	})
	return err
}

func (s *Service) ListOperationLogs(ctx context.Context, limit int) ([]OperationAuditLog, error) {
	return s.repo.ListOperationLogs(ctx, limit)
}
