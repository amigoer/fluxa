package service

import (
	"github.com/amigoer/fluxa/internal/provider/health"
	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/repo"
	"github.com/amigoer/fluxa/internal/provider/routing"
)

// Service is the provider module's business layer, composed of one
// sub-interface per feature. Each sub-interface is declared and
// implemented in its own file next to this one; this file only assembles
// them and holds the single implementation they all share.
type Service interface {
	ProviderService
	ModelService
	ProcurementService
	RoutingService
	VirtualKeyService
	QuotaPoolService
	QuotaRequestService
	Runtime
}

type service struct {
	repo     repo.Repo
	breaker  *health.Breaker
	keys     *keyauth.Cache
	resolver *routing.Resolver
}

func New(repo repo.Repo) Service {
	breaker := health.NewBreaker(repo)
	return &service{
		repo:     repo,
		breaker:  breaker,
		keys:     keyauth.NewCache(repo),
		resolver: routing.NewResolver(repo, repo, breaker),
	}
}
