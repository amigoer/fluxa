package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/security/types"
)

// RuleService maintains the DLP rule set behind the 数据安全规则 page.
type RuleService interface {
	CreateRule(ctx context.Context, rule types.DLPRule) (types.DLPRule, error)
	ListRules(ctx context.Context) ([]types.DLPRule, error)
	SetRuleEnabled(ctx context.Context, id string, enabled bool) error
	DeleteRule(ctx context.Context, id string) error
}

func (s *service) CreateRule(ctx context.Context, rule types.DLPRule) (types.DLPRule, error) {
	return s.repo.CreateRule(ctx, rule)
}

func (s *service) ListRules(ctx context.Context) ([]types.DLPRule, error) {
	return s.repo.ListRules(ctx)
}

func (s *service) SetRuleEnabled(ctx context.Context, id string, enabled bool) error {
	return s.repo.UpdateRuleEnabled(ctx, id, enabled)
}

func (s *service) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}
