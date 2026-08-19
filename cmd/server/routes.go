package main

import (
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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
	userhandler "github.com/amigoer/fluxa/internal/user/handler"
	userrepo "github.com/amigoer/fluxa/internal/user/repo"
	userservice "github.com/amigoer/fluxa/internal/user/service"
	usersession "github.com/amigoer/fluxa/internal/user/session"
)

const (
	// consoleRequestTimeout bounds anything the web UI calls.
	consoleRequestTimeout = 60 * time.Second

	// gatewayRequestTimeout bounds a proxied completion. It sits above
	// the upstream client's own 5-minute deadline on purpose, so that
	// deadline stays the one that actually governs and this only ever
	// catches a request that escaped it.
	gatewayRequestTimeout = 10 * time.Minute
)

// wireRoutes mounts every module's HTTP routes onto r. Each module owns
// its own handler package; this function is just the assembly point, filled
// in as each module lands.
//
// It returns the provider service because one thing outside the request
// path needs it: the quota-reservation sweeper in main.
func wireRoutes(r chi.Router, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) providerservice.Service {
	userRepo := userrepo.New(pool)
	userService := userservice.New(userRepo)
	sessions := usersession.NewManager(userRepo, userService, cfg.SessionCookieName, cfg.SessionCookieSecure)
	userHandler := userhandler.New(userService, userRepo, sessions, cfg.BaseURL)

	providerRepo := providerrepo.New(pool)
	providerService := providerservice.New(providerRepo)
	providerHandler := providerhandler.New(providerService, providerRepo)

	securityRepo := securityrepo.New(pool)
	securityService := securityservice.New(securityRepo)
	securityHandler := securityhandler.New(securityService)

	auditRepo := auditrepo.New(pool)
	auditService := auditservice.New(auditRepo)
	auditHandler := audithandler.New(auditService)

	// Everything a browser talks to -- the sign-in endpoints and the
	// management API alike. A request here is a page load or a form
	// submit and has no business running for a minute, so the whole
	// group is bounded; the gateway below deliberately is not.
	r.Group(func(r chi.Router) {
		r.Use(chimiddleware.Timeout(consoleRequestTimeout))

		userHandler.RegisterPublicRoutes(r)

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
	})

	// The actual gateway: virtual-key (bearer token) authenticated, not
	// session-cookie authenticated -- this is what an integration calls
	// programmatically, not a browser.
	//
	// Its timeout is a backstop, not the working limit: the upstream
	// client's own 5-minute deadline is what a slow provider hits first,
	// and this only exists so a call that somehow outlives that cannot
	// pin a goroutine and its response writer indefinitely.
	r.Group(func(r chi.Router) {
		r.Use(chimiddleware.Timeout(gatewayRequestTimeout))

		pipeline := gateway.NewPipeline(providerService, securityService, auditService, cfg.MaxRequestCostMicroCents)
		pipeline.RegisterRoutes(r)
	})

	return providerService
}
