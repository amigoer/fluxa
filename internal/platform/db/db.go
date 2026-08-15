// Package db wires up the PostgreSQL connection pool and runs schema
// migrations. All data access in the rest of the codebase goes through
// pgx directly (hand-written SQL in each module's repo.go); this package
// only owns the pool lifecycle and migration bootstrap.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a connection pool for the given DSN and verifies
// connectivity with a ping.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}
