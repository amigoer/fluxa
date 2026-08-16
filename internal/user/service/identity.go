package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// IdentityService owns the per-provider app credentials an external
// sign-in source needs (Feishu today; WeCom and DingTalk are wired the
// same way when they land).
type IdentityService interface {
	GetIdentityConfig(ctx context.Context, provider types.IdentityProvider) (types.IdentityConfig, error)
	UpsertIdentityConfig(ctx context.Context, c types.IdentityConfig) error
}

func (s *service) GetIdentityConfig(ctx context.Context, provider types.IdentityProvider) (types.IdentityConfig, error) {
	return s.repo.GetIdentityConfig(ctx, provider)
}

func (s *service) UpsertIdentityConfig(ctx context.Context, c types.IdentityConfig) error {
	return s.repo.UpsertIdentityConfig(ctx, c)
}
