// bootstrap.go bridges the persistent store rows and the in-memory
// config.ProviderConfig / config.RouteConfig types consumed by the router.
// It also handles the "first run" flow where the operator still has a YAML
// file — we copy any providers/routes it declares into the database so the
// gateway starts with its legacy configuration intact.

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
// It is the form used when the admin API ingests a JSON payload or when
// seed data is copied from YAML into the database.
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

// SeedIfEmpty copies the providers and routes sections of cfg into the
// store if and only if both tables are currently empty. Returns true when
// a seed actually happened. Subsequent restarts leave the database
// untouched so admin edits are never clobbered by the YAML file.
func (s *Store) SeedIfEmpty(ctx context.Context, cfg config.Config) (bool, error) {
	provs, err := s.ListProviders(ctx)
	if err != nil {
		return false, err
	}
	routes, err := s.ListRoutes(ctx)
	if err != nil {
		return false, err
	}
	if len(provs) > 0 || len(routes) > 0 {
		return false, nil
	}
	for _, pc := range cfg.Providers {
		if err := s.UpsertProvider(ctx, FromConfigProvider(pc)); err != nil {
			return false, err
		}
	}
	for _, rc := range cfg.Routes {
		if err := s.UpsertRoute(ctx, FromConfigRoute(rc)); err != nil {
			return false, err
		}
	}
	return len(cfg.Providers) > 0 || len(cfg.Routes) > 0, nil
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
