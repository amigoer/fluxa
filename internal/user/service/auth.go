package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// AuthSettingsService owns the deployment-wide login policy: whether the
// local phone/email path is open at all, and whether a registration
// waits for an admin's approval.
type AuthSettingsService interface {
	GetAuthSettings(ctx context.Context) (types.AuthSettings, error)
	UpdateAuthSettings(ctx context.Context, settings types.AuthSettings) error
}

func (s *service) GetAuthSettings(ctx context.Context) (types.AuthSettings, error) {
	return s.repo.GetAuthSettings(ctx)
}

func (s *service) UpdateAuthSettings(ctx context.Context, settings types.AuthSettings) error {
	return s.repo.UpdateAuthSettings(ctx, settings)
}
