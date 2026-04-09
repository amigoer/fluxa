// Package main is the entrypoint for the Fluxa AI gateway binary.
//
// The binary is intentionally tiny: it reads its runtime configuration
// from environment variables, opens the SQLite store that holds
// providers, routes, and virtual keys, wires up the router plus admin
// REST surface, and starts the HTTP server. Every piece of real logic
// lives under internal/ so other entrypoints (CLI, tests, embedded
// library use) can compose the same building blocks.
//
// Starting with v2.1 there is no YAML file on the startup path. The
// `fluxa` binary boots on a fresh box with zero files — operators
// configure providers through the dashboard at / or the /admin REST
// API, and can still round-trip the full state as YAML through the
// /admin/config/export and /admin/config/import endpoints.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/amigoer/fluxa/internal/api"
	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/keys"
	"github.com/amigoer/fluxa/internal/router"
	"github.com/amigoer/fluxa/internal/store"
	fluxaweb "github.com/amigoer/fluxa/web"
)

// Version is the gateway release version. It is overridden at build time
// via -ldflags "-X main.Version=...".
var Version = "0.0.1-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("fluxa %s\n", Version)
		return
	}

	// The only flag left is a convenience override for the log level so
	// `fluxa -log-level=debug` still works without exporting an env var.
	// Everything else is an env setting — see internal/config/env.go.
	logLevel := flag.String("log-level", "", "log level override: debug, info, warn, error")
	flag.Parse()

	cfg := config.FromEnv()
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}

	logger := newLogger(cfg.Logging.Level)
	slog.SetDefault(logger)

	// Open the SQLite-backed config store. Providers, routes, virtual
	// keys and usage rows all live here so operators can mutate them
	// through the admin API without restarting the process.
	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		logger.Error("open store", "path", cfg.Database.Path, "err", err)
		os.Exit(1)
	}
	defer st.Close()

	provs, routes, err := st.LoadRouterInputs(context.Background())
	if err != nil {
		logger.Error("load router inputs", "err", err)
		os.Exit(1)
	}
	// Zero providers is non-fatal: the admin dashboard at / still boots
	// so operators can configure their first provider live. The data-plane
	// /v1/* endpoints return errors until a provider exists, which is the
	// expected behaviour on a brand new install.
	if len(provs) == 0 {
		logger.Warn("no providers configured — data plane is idle; open the dashboard or use the admin API to add one")
	}

	r := router.New()
	r.SetStore(st)
	r.SetLogger(logger)
	if err := r.Reload(provs, routes); err != nil && len(provs) > 0 {
		logger.Error("build router", "err", err)
		os.Exit(1)
	}
	// Load virtual model and regex model tables into the resolver's
	// snapshot. Failures here are non-fatal: a brand new install will
	// have empty tables and the resolver simply passes every request
	// through to the legacy provider chain.
	if err := r.ReloadVirtualModels(context.Background()); err != nil {
		logger.Warn("load virtual models", "err", err)
	}
	if err := r.ReloadRegexModels(context.Background()); err != nil {
		logger.Warn("load regex models", "err", err)
	}

	// Build the virtual-key runtime. An empty ring is the "legacy / no
	// virtual keys" mode, so failure to reload from the store is fatal —
	// silently running without auth would be surprising.
	kr := keys.NewKeyring(st)
	if err := kr.Reload(context.Background()); err != nil {
		logger.Error("load virtual keys", "err", err)
		os.Exit(1)
	}

	// First-run bootstrap: if there are no admin users in the store yet,
	// seed a default account so a fresh deployment can sign in. The
	// credentials come from FLUXA_BOOTSTRAP_USER / FLUXA_BOOTSTRAP_PASSWORD
	// (defaults: admin / admin) and operators are nudged in the logs to
	// rotate the password through the dashboard immediately.
	if n, err := st.CountAdminUsers(context.Background()); err != nil {
		logger.Error("count admin users", "err", err)
		os.Exit(1)
	} else if n == 0 {
		bootstrapUser := getEnvDefault("FLUXA_BOOTSTRAP_USER", "admin")
		bootstrapPass := getEnvDefault("FLUXA_BOOTSTRAP_PASSWORD", "admin")
		if _, err := st.CreateAdminUser(context.Background(), bootstrapUser, bootstrapPass); err != nil {
			logger.Error("bootstrap admin user", "err", err)
			os.Exit(1)
		}
		logger.Warn("seeded default admin account — change the password immediately",
			"username", bootstrapUser)
	}
	// Best-effort cleanup of any sessions that expired while the gateway
	// was offline so the table does not grow forever.
	_ = st.PurgeExpiredSessions(context.Background())

	mux := http.NewServeMux()
	api.New(r, logger, kr, st).Routes(mux)
	api.NewAdmin(r, st, kr, logger).Routes(mux)

	// Mount the embedded admin dashboard at the root. The handler is a
	// catch-all for GET requests, so anything that does not match a more
	// specific pattern (the /v1/* data plane, the /admin/* control
	// plane, /health) falls through to the SPA — which in turn serves
	// index.html for unknown paths so client-side routes keep working.
	mux.Handle("GET /", fluxaweb.Handler("/"))

	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("fluxa starting", "addr", addr, "version", Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			logger.Error("server crashed", "err", err)
			os.Exit(1)
		}
		return
	}

	// Graceful shutdown: give in-flight requests a deadline to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "err", err)
		os.Exit(1)
	}
	logger.Info("fluxa stopped cleanly")
}

// getEnvDefault is a tiny helper used by the bootstrap path to read an
// optional env var with a fallback. The full FromEnv loader lives under
// internal/config but the bootstrap user/password values do not belong
// in Runtime — they only matter the first time the gateway boots.
func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newLogger builds a JSON slog.Logger at the requested level. Invalid level
// strings fall back to info so a typo never keeps the binary from starting.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Normalise the timestamp to RFC3339 for stable log ingestion.
			if a.Key == slog.TimeKey {
				t, ok := a.Value.Any().(time.Time)
				if ok {
					return slog.String("time", t.UTC().Format(time.RFC3339Nano))
				}
			}
			return a
		},
	})
	return slog.New(handler)
}
