// Package main is the entrypoint for the Fluxa AI gateway binary.
//
// The binary is intentionally tiny: it parses flags, loads configuration,
// builds the router, and starts the HTTP server. Every piece of real logic
// lives under internal/ so that other entrypoints (CLI, tests, embedded
// library use) can compose the same building blocks.
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

	configPath := flag.String("config", "fluxa.yaml", "path to YAML configuration file")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	flag.Parse()

	logger := newLogger(*logLevel)
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "path", *configPath, "err", err)
		os.Exit(1)
	}

	// Open the SQLite-backed config store. Providers and routes live here
	// so operators can mutate them through the admin API without editing
	// YAML and bouncing the process.
	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		logger.Error("open store", "path", cfg.Database.Path, "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// First-run ergonomics: if the store is empty, seed it from the
	// providers/routes sections of the YAML file. Subsequent starts never
	// touch the rows again so admin edits are preserved.
	if seeded, err := st.SeedIfEmpty(context.Background(), cfg); err != nil {
		logger.Error("seed store", "err", err)
		os.Exit(1)
	} else if seeded {
		logger.Info("seeded store from yaml", "providers", len(cfg.Providers), "routes", len(cfg.Routes))
	}

	provs, routes, err := st.LoadRouterInputs(context.Background())
	if err != nil {
		logger.Error("load router inputs", "err", err)
		os.Exit(1)
	}
	if len(provs) == 0 {
		logger.Error("no providers configured", "hint", "seed providers via YAML or the admin API")
		os.Exit(1)
	}

	r := router.New()
	if err := r.Reload(provs, routes); err != nil {
		logger.Error("build router", "err", err)
		os.Exit(1)
	}

	// Build the virtual-key runtime. An empty ring is the "legacy / no
	// virtual keys" mode, so failure to reload from the store is fatal —
	// silently running without auth would be surprising.
	kr := keys.NewKeyring(st)
	if err := kr.Reload(context.Background()); err != nil {
		logger.Error("load virtual keys", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	api.New(r, logger, kr, st).Routes(mux)
	api.NewAdmin(r, st, kr, cfg.Server.MasterKey, logger).Routes(mux)

	// Mount the embedded admin dashboard at /ui/. A bare /ui (no
	// trailing slash) redirects so relative asset paths resolve.
	mux.Handle("GET /ui/", fluxaweb.Handler("/ui/"))
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
	})

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
