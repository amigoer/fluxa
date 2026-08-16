package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// RoutingService edits the routing chains the gateway resolves against.
// Resolving one is not here but on Runtime, which hands out the stateful
// routing.Resolver the hot path uses.
type RoutingService interface {
	CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error)
	ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error)
}

func (s *service) CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error) {
	return s.repo.CreateRoutingRule(ctx, rule)
}

func (s *service) ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error) {
	return s.repo.ListRoutingChain(ctx, scope, ownerMemberID)
}
