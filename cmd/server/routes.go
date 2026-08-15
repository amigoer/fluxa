package main

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/platform/config"
)

// wireRoutes mounts every module's HTTP routes onto r. Each module owns
// its own handler.go; this function is just the assembly point, filled
// in as each module lands.
func wireRoutes(r chi.Router, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) {
	_ = cfg
	_ = pool
	_ = log
}
