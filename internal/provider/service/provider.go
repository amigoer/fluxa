package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// ProviderService manages the upstream vendors the gateway forwards to.
type ProviderService interface {
	CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error)
	ListProviders(ctx context.Context, orgID string) ([]types.Provider, error)
	GetProvider(ctx context.Context, id string) (types.Provider, error)
}

// ErrProviderKindUnsupported is a kind the schema accepts but the
// gateway cannot forward to yet.
var ErrProviderKindUnsupported = errors.New("provider: this provider kind is not supported yet")

func (s *service) CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error) {
	if !p.Kind.Implemented() {
		return types.Provider{}, fmt.Errorf("%w: %s", ErrProviderKindUnsupported, p.Kind)
	}
	p.Status = types.ProviderStatusActive
	created, err := s.repo.CreateProvider(ctx, p)
	if err != nil {
		return types.Provider{}, err
	}
	// Every provider starts healthy; seed its breaker row so
	// health.Breaker never has to special-case "no row yet".
	if _, err := s.repo.GetProviderHealth(ctx, created.ID); err != nil {
		return types.Provider{}, err
	}
	return created, nil
}

func (s *service) ListProviders(ctx context.Context, orgID string) ([]types.Provider, error) {
	return s.repo.ListProviders(ctx, orgID)
}

func (s *service) GetProvider(ctx context.Context, id string) (types.Provider, error) {
	return s.repo.GetProvider(ctx, id)
}
