package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/security/types"
)

// EventService is the trail behind the 安全事件 page: what fired, for
// whom, and what was done about it.
type EventService interface {
	LogEvent(ctx context.Context, memberID, virtualKeyID *string, hit Hit) error
	ListEvents(ctx context.Context, limit int) ([]types.SecurityEvent, error)
}

// LogEvent records that a rule fired, for the 安全事件 page.
func (s *service) LogEvent(ctx context.Context, memberID, virtualKeyID *string, hit Hit) error {
	description := hit.Rule.Name + "已" + actionVerb(hit.Rule.Action)
	_, err := s.repo.LogEvent(ctx, types.SecurityEvent{
		MemberID:     memberID,
		VirtualKeyID: virtualKeyID,
		RuleID:       &hit.Rule.ID,
		Description:  description,
		ActionTaken:  hit.Rule.Action,
	})
	return err
}

func actionVerb(a types.RuleAction) string {
	if a == types.ActionBlock {
		return "拦截"
	}
	return "脱敏"
}

func (s *service) ListEvents(ctx context.Context, limit int) ([]types.SecurityEvent, error) {
	return s.repo.ListEvents(ctx, limit)
}
