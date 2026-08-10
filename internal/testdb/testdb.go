// Package testdb provisions throwaway Postgres databases for tests.
//
// Fluxa talks to Postgres and nothing else, so the store, keys and api
// test suites all need a real server. Point FLUXA_TEST_DATABASE_URL at
// one and every database-backed test runs against it:
//
//	FLUXA_TEST_DATABASE_URL=postgres://fluxa:fluxa@localhost:5432/fluxa_test?sslmode=disable go test ./...
//
// Without that variable the database-backed tests skip, so `go test
// ./...` still passes on a machine with no Postgres — the pure-logic
// suites (adapters, router, config, pricing) cover themselves.
//
// Each caller gets its own freshly created schema and a DSN whose
// search_path points at it, so parallel tests never see each other's
// rows and a failed run leaves nothing behind but one dropped schema.
package testdb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/amigoer/fluxa/internal/config"
)

// EnvVar is the environment variable holding the DSN of the Postgres
// instance tests may create schemas in.
const EnvVar = "FLUXA_TEST_DATABASE_URL"

// New returns a DatabaseConfig pointing at a private schema on the test
// server, and skips the calling test when no test server is configured.
// The schema is dropped when the test finishes.
func New(t *testing.T) config.DatabaseConfig {
	t.Helper()

	dsn := os.Getenv(EnvVar)
	if dsn == "" {
		t.Skipf("%s not set — skipping Postgres-backed test", EnvVar)
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("testdb: open %s: %v", EnvVar, err)
	}
	if err := admin.Ping(); err != nil {
		admin.Close()
		t.Fatalf("testdb: connect to the server in %s: %v", EnvVar, err)
	}

	schema := "fluxa_test_" + randomSuffix()
	// The identifier is generated here, never caller-supplied, so
	// interpolating it into the DDL cannot smuggle anything in.
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		admin.Close()
		t.Fatalf("testdb: create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`); err != nil {
			t.Logf("testdb: drop schema %s: %v", schema, err)
		}
		admin.Close()
	})

	return config.DatabaseConfig{
		URL: scopedDSN(t, dsn, schema),
		// A handful of connections is plenty for a test and keeps a
		// package-wide `go test` from exhausting max_connections.
		MaxOpenConns: 4,
		MaxIdleConns: 2,
	}
}

// scopedDSN rewrites the server DSN so every connection made with it
// starts with search_path set to the test's own schema. The compact
// "-csearch_path=..." form avoids embedding a space in the URL.
func scopedDSN(t *testing.T, dsn, schema string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("testdb: %s is not a valid URL: %v", EnvVar, err)
	}
	q := u.Query()
	q.Set("options", "-csearch_path="+schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func randomSuffix() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand does not fail in practice; a fixed suffix would
		// only collide with a concurrent run, which the caller notices
		// as a "schema already exists" error.
		return "fallback"
	}
	return hex.EncodeToString(buf[:])
}
