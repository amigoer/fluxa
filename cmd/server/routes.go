package main

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	audithandler "github.com/amigoer/fluxa/internal/audit/handler"
	auditrepo "github.com/amigoer/fluxa/internal/audit/repo"
	auditservice "github.com/amigoer/fluxa/internal/audit/service"
	"github.com/amigoer/fluxa/internal/gateway"
	"github.com/amigoer/fluxa/internal/platform/config"
	providerhandler "github.com/amigoer/fluxa/internal/provider/handler"
	providerrepo "github.com/amigoer/fluxa/internal/provider/repo"
	providerservice "github.com/amigoer/fluxa/internal/provider/service"
	securityhandler "github.com/amigoer/fluxa/internal/security/handler"
	securityrepo "github.com/amigoer/fluxa/internal/security/repo"
	securityservice "github.com/amigoer/fluxa/internal/security/service"
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
	userHandler.RegisterPublicRoutes(r)

	providerRepo := providerrepo.New(pool)
	providerService := providerservice.New(providerRepo)
	providerHandler := providerhandler.New(providerService, providerRepo)

	securityRepo := securityrepo.New(pool)
	securityService := securityservice.New(securityRepo)
	securityHandler := securityhandler.New(securityService)

	auditRepo := auditrepo.New(pool)
	auditService := auditservice.New(auditRepo)
	auditHandler := audithandler.New(auditService)

	// Session-authenticated management API: everything an admin or
	// employee reaches through the web UI. Every module's protected
	// routes share this one group so they share its middleware -- in
	// particular RecordMutations, which writes the operation audit trail
	// for whatever is mounted here and therefore cannot be forgotten by a
	// handler that lands later.
	r.Group(func(r chi.Router) {
		r.Use(sessions.Middleware)
		r.Use(auditService.RecordMutations)

		userHandler.RegisterProtectedRoutes(r)
		providerHandler.RegisterRoutes(r)
		securityHandler.RegisterRoutes(r)
		auditHandler.RegisterRoutes(r)
	})

	// The actual gateway: virtual-key (bearer token) authenticated, not
	// session-cookie authenticated -- this is what an integration calls
	// programmatically, not a browser.
	pipeline := gateway.NewPipeline(providerService, securityService, auditService)
	pipeline.RegisterRoutes(r)
}
