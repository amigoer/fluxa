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
	"github.com/amigoer/fluxa/internal/router"
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

	r, err := router.Build(cfg)
	if err != nil {
		logger.Error("build router", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	api.New(r, logger).Routes(mux)

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
