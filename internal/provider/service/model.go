package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ModelService manages the models published on top of a provider: the
// full catalogue for admins, and the published subset employees pick
// from.
type ModelService interface {
	CreateModel(ctx context.Context, m types.Model) (types.Model, error)
	GetModel(ctx context.Context, id string) (types.Model, error)
	ListModels(ctx context.Context, orgID string) ([]types.Model, error)
	ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error)
	ListModelsForVirtualKey(ctx context.Context, virtualKeyID string) ([]types.Model, error)
}

func (s *service) CreateModel(ctx context.Context, m types.Model) (types.Model, error) {
	return s.repo.CreateModel(ctx, m)
}

func (s *service) GetModel(ctx context.Context, id string) (types.Model, error) {
	return s.repo.GetModel(ctx, id)
}

func (s *service) ListModels(ctx context.Context, orgID string) ([]types.Model, error) {
	return s.repo.ListModels(ctx, orgID)
}

func (s *service) ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error) {
	return s.repo.ListPublishedModels(ctx, orgID)
}

// ListModelsForVirtualKey backs the gateway's /v1/models: what this key
// may call, not what the deployment has.
func (s *service) ListModelsForVirtualKey(ctx context.Context, virtualKeyID string) ([]types.Model, error) {
	return s.repo.ListModelsForVirtualKey(ctx, virtualKeyID)
}
