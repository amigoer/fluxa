// bootstrap.go bridges the persistent store rows and the in-memory
// config.ProviderConfig / config.RouteConfig types consumed by the router.
// It also exposes the Import entry point used by the admin YAML import
// endpoint to replay a Bundle into the database.

package store

import (
	"context"
	"time"

	"github.com/amigoer/fluxa/internal/config"
)

// ToConfigProviders converts store rows to the runtime ProviderConfig type
// understood by the router. Disabled rows are filtered out so operators can
// park a credential without deleting it.
func ToConfigProviders(rows []Provider) []config.ProviderConfig {
	out := make([]config.ProviderConfig, 0, len(rows))
	for _, p := range rows {
		if !p.Enabled {
			continue
		}
		timeout := time.Duration(0)
		if p.TimeoutSec > 0 {
			timeout = time.Duration(p.TimeoutSec) * time.Second
		}
		out = append(out, config.ProviderConfig{
			Name:         p.Name,
			Kind:         p.Kind,
			APIKey:       p.APIKey,
			BaseURL:      p.BaseURL,
			APIVersion:   p.APIVersion,
			Region:       p.Region,
			AccessKey:    p.AccessKey,
			SecretKey:    p.SecretKey,
			SessionToken: p.SessionToken,
			Deployments:  p.Deployments,
			Models:       p.Models,
			Headers:      p.Headers,
			Timeout:      timeout,
		})
	}
	return out
}

// ToConfigRoutes converts store rows to the runtime RouteConfig type.
func ToConfigRoutes(rows []Route) []config.RouteConfig {
	out := make([]config.RouteConfig, 0, len(rows))
	for _, r := range rows {
		out = append(out, config.RouteConfig{
			Model:    r.Model,
			Provider: r.Provider,
			Fallback: r.Fallback,
		})
	}
	return out
}

// FromConfigProvider is the inverse of ToConfigProviders for a single row.
// It is used by the admin API ingest path and by Import when replaying a
// YAML bundle.
func FromConfigProvider(pc config.ProviderConfig) Provider {
	return Provider{
		Name:         pc.Name,
		Kind:         pc.Kind,
		APIKey:       pc.APIKey,
		BaseURL:      pc.BaseURL,
		APIVersion:   pc.APIVersion,
		Region:       pc.Region,
		AccessKey:    pc.AccessKey,
		SecretKey:    pc.SecretKey,
		SessionToken: pc.SessionToken,
		Deployments:  pc.Deployments,
		Models:       pc.Models,
		Headers:      pc.Headers,
		TimeoutSec:   int(pc.Timeout / time.Second),
		Enabled:      true,
	}
}

// FromConfigRoute is the inverse of ToConfigRoutes for a single row.
func FromConfigRoute(rc config.RouteConfig) Route {
	return Route{Model: rc.Model, Provider: rc.Provider, Fallback: rc.Fallback}
}

// Import upserts every provider and route from a YAML bundle into the
// store unconditionally. Unlike the old SeedIfEmpty flow it never inspects
// the current row count: operators who hit /admin/config/import explicitly
// asked for the incoming document to win. Existing rows not mentioned in
// the bundle are left alone so imports can be additive.
func (s *Store) Import(ctx context.Context, providers []config.ProviderConfig, routes []config.RouteConfig) error {
	for _, pc := range providers {
		if err := s.UpsertProvider(ctx, FromConfigProvider(pc)); err != nil {
			return err
		}
	}
	for _, rc := range routes {
		if err := s.UpsertRoute(ctx, FromConfigRoute(rc)); err != nil {
			return err
		}
	}
	return nil
}

// LoadRouterInputs fetches the enabled providers and all routes from the
// store and returns them in the slice form expected by router.Reload. It is
// the one call that main (and the admin API after a write) needs to make to
// rebuild the router state.
func (s *Store) LoadRouterInputs(ctx context.Context) ([]config.ProviderConfig, []config.RouteConfig, error) {
	provs, err := s.ListProviders(ctx)
	if err != nil {
		return nil, nil, err
	}
	routes, err := s.ListRoutes(ctx)
	if err != nil {
		return nil, nil, err
	}
	return ToConfigProviders(provs), ToConfigRoutes(routes), nil
}
