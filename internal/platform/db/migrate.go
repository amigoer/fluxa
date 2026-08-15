package db

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/amigoer/fluxa/migrations"
)

// Migrate applies every pending migration embedded in the migrations
// package. It is safe to call on every startup: golang-migrate tracks
// the applied version in the schema_migrations table and is a no-op
// once the schema is current.
func Migrate(dsn string) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("db: load migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, toMigrateDSN(dsn))
	if err != nil {
		return fmt.Errorf("db: init migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: apply migrations: %w", err)
	}

	return nil
}

// toMigrateDSN rewrites a standard postgres:// / postgresql:// connection
// string to the pgx5:// scheme the golang-migrate pgx/v5 driver expects,
// so callers only ever have to configure one, standard DSN.
func toMigrateDSN(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + strings.TrimPrefix(dsn, prefix)
		}
	}
	return dsn
}
