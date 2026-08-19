// Command server is the single executable that runs Fluxa: it wires up
// the User, Provider and Security modules plus their supporting layers
// (gateway, audit, notify, rbac, platform) and serves both the admin API
// and the embedded frontend from one process (see DESIGN.md section 12).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/platform/config"
	"github.com/amigoer/fluxa/internal/platform/db"
	"github.com/amigoer/fluxa/internal/platform/logger"
	providerservice "github.com/amigoer/fluxa/internal/provider/service"
	"github.com/amigoer/fluxa/web"
)

func main() {
	log := logger.New()
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router, providerService, err := newRouter(cfg, pool, log)
	if err != nil {
		return err
	}

	sweeperCtx, stopSweeper := context.WithCancel(ctx)
	defer stopSweeper()
	go sweepQuotaReservations(sweeperCtx, providerService, log)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// newRouter builds the full route tree. Route registration is added
// module by module as each one is implemented; see wireRoutes. The
// embedded frontend is mounted last, as the catch-all for anything that
// isn't an /api or /v1 route, so client-side routes like
// /admin/overview fall through to index.html instead of 404ing.
func newRouter(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (http.Handler, providerservice.Service, error) {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	// No blanket request timeout here on purpose. A 60s ceiling over
	// every route also covered /v1, where a single completion running
	// for minutes is the normal case, not a hung request -- long
	// documents, reasoning models and agent loops all sat well past it
	// and had their upstream call cancelled mid-stream. Each route group
	// now sets the timeout that suits it; see wireRoutes.

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	providerService := wireRoutes(r, cfg, pool, log)

	frontend, err := web.Handler()
	if err != nil {
		return nil, nil, err
	}
	r.NotFound(frontend.ServeHTTP)

	return r, providerService, nil
}

// reservationSweepInterval is how often abandoned quota reservations are
// released. It only has to be well under the reservation TTL: the sweep
// is the backstop for a call that died without settling, and until it
// runs that call's budget stays promised to nothing.
const reservationSweepInterval = time.Minute

// sweepQuotaReservations releases budget promised to calls that never
// settled -- a process killed mid-flight, a panic that unwound past the
// deferred release. Without it a crash permanently shrinks the key's
// usable budget, so this is part of the reservation scheme working, not
// a cleanup nicety.
func sweepQuotaReservations(ctx context.Context, providers providerservice.Service, log *slog.Logger) {
	ticker := time.NewTicker(reservationSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			freed, err := providers.ExpireStaleReservations(ctx)
			if err != nil {
				log.Error("sweep quota reservations", "error", err)
				continue
			}
			if freed > 0 {
				log.Warn("released quota reservations left behind by calls that never settled",
					"count", freed)
			}
		}
	}
}
