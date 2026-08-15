package main

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/platform/config"
	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/user"
)

// wireRoutes mounts every module's HTTP routes onto r. Each module owns
// its own handler.go; this function is just the assembly point, filled
// in as each module lands.
func wireRoutes(r chi.Router, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) {
	userRepo := user.NewRepo(pool)
	userService := user.NewService(userRepo)
	sessions := user.NewSessionManager(userRepo, userService, cfg.SessionCookieName, cfg.SessionCookieSecure)
	userHandler := user.NewHandler(userService, userRepo, sessions, cfg.BaseURL)
	userHandler.RegisterRoutes(r)

	providerRepo := provider.NewRepo(pool)
	providerService := provider.NewService(providerRepo)
	providerHandler := provider.NewHandler(providerService, providerRepo)
	r.Group(func(r chi.Router) {
		r.Use(sessions.Middleware)
		providerHandler.RegisterRoutes(r)
	})
}
