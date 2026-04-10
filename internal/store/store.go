// Package store persists Fluxa provider and route configuration in a
// SQLite database. It replaces the static YAML provider/route sections so
// operators can mutate the gateway live through the admin API without
// restarting the process.
//
// The store uses modernc.org/sqlite, a pure-Go driver, so the single-binary
// distribution still builds with CGO_ENABLED=0.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a lookup by primary key fails.
var ErrNotFound = errors.New("store: not found")

// Store wraps a *sql.DB handle and exposes typed CRUD helpers for the
// providers and routes tables.
type Store struct {
	db *sql.DB
}

// Provider mirrors the providers table row. It is the persistent form of
// config.ProviderConfig and is converted to/from the richer runtime type by
// the caller.
type Provider struct {
	Name         string
	Kind         string
	APIKey       string
	BaseURL      string
	APIVersion   string
	Region       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	Deployments  map[string]string
	Models       []string
	Headers      map[string]string
	TimeoutSec   int
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Route mirrors the routes table row. It is the persistent form of a
// "this user-facing model name → that provider, with these fallbacks"
// rule. The richer "alias under multiple real models with weighted
// traffic split" use case is served by virtual_models, not by Route.
type Route struct {
	Model     string
	Provider  string
	Fallback  []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Open opens (and migrates) the SQLite database at path. Pass ":memory:"
// for a transient in-process database, useful in tests.
func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if path == ":memory:" {
		dsn = path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite writer serialisation; readers still go through WAL.
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for advanced callers (tests, admin tooling).
func (s *Store) DB() *sql.DB { return s.db }

// migrate creates the schema on an empty database. The migrations are
// idempotent; every statement uses IF NOT EXISTS so restarting on an
// existing database is a no-op.
func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			name            TEXT PRIMARY KEY,
			kind            TEXT NOT NULL,
			api_key         TEXT NOT NULL DEFAULT '',
			base_url        TEXT NOT NULL DEFAULT '',
			api_version     TEXT NOT NULL DEFAULT '',
			region          TEXT NOT NULL DEFAULT '',
			access_key      TEXT NOT NULL DEFAULT '',
			secret_key      TEXT NOT NULL DEFAULT '',
			session_token   TEXT NOT NULL DEFAULT '',
			deployments     TEXT NOT NULL DEFAULT '{}',
			models          TEXT NOT NULL DEFAULT '[]',
			headers         TEXT NOT NULL DEFAULT '{}',
			timeout_sec     INTEGER NOT NULL DEFAULT 0,
			enabled         INTEGER NOT NULL DEFAULT 1,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS routes (
			model           TEXT PRIMARY KEY,
			provider        TEXT NOT NULL,
			fallback        TEXT NOT NULL DEFAULT '[]',
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (provider) REFERENCES providers(name) ON UPDATE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_routes_provider ON routes(provider)`,
		`CREATE TABLE IF NOT EXISTS virtual_keys (
			id                    TEXT PRIMARY KEY,
			name                  TEXT NOT NULL,
			description           TEXT NOT NULL DEFAULT '',
			models                TEXT NOT NULL DEFAULT '[]',
			ip_allowlist          TEXT NOT NULL DEFAULT '[]',
			budget_tokens_daily   INTEGER NOT NULL DEFAULT 0,
			budget_tokens_monthly INTEGER NOT NULL DEFAULT 0,
			budget_usd_daily      REAL NOT NULL DEFAULT 0,
			budget_usd_monthly    REAL NOT NULL DEFAULT 0,
			rpm_limit             INTEGER NOT NULL DEFAULT 0,
			enabled               INTEGER NOT NULL DEFAULT 1,
			expires_at            DATETIME,
			created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS usage_records (
			id                 INTEGER PRIMARY KEY AUTOINCREMENT,
			virtual_key_id     TEXT NOT NULL,
			ts                 DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			model              TEXT NOT NULL,
			provider           TEXT NOT NULL,
			prompt_tokens      INTEGER NOT NULL DEFAULT 0,
			completion_tokens  INTEGER NOT NULL DEFAULT 0,
			total_tokens       INTEGER NOT NULL DEFAULT 0,
			cost_usd           REAL NOT NULL DEFAULT 0,
			latency_ms         INTEGER NOT NULL DEFAULT 0,
			status             INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (virtual_key_id) REFERENCES virtual_keys(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_vk_ts ON usage_records(virtual_key_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_ts ON usage_records(ts)`,
		// admin_users / admin_sessions back the dashboard login flow.
		// Passwords are bcrypt hashes — never store anything reversible
		// here. Sessions are opaque random tokens with a TTL; the
		// requireAuth middleware joins on user_id to look up the caller.
		`CREATE TABLE IF NOT EXISTS admin_users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (
			token       TEXT PRIMARY KEY,
			user_id     INTEGER NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at  DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON admin_sessions(expires_at)`,
		// virtual_models is the "user-facing model name" alias table
		// added in v2.4. A virtual model fans out to one or more real
		// (or virtual) targets with weighted traffic splitting; the
		// resolver in internal/router/model_resolver.go evaluates the
		// chain at request time. ON DELETE CASCADE on the child table
		// keeps an admin "delete virtual model" call from leaving
		// orphaned route rows behind.
		`CREATE TABLE IF NOT EXISTS virtual_models (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS virtual_model_routes (
			id               TEXT PRIMARY KEY,
			virtual_model_id TEXT NOT NULL,
			weight           INTEGER NOT NULL CHECK(weight > 0),
			target_type      TEXT NOT NULL CHECK(target_type IN ('real','virtual')),
			target_model     TEXT NOT NULL,
			provider         TEXT NOT NULL DEFAULT '',
			enabled          INTEGER NOT NULL DEFAULT 1,
			position         INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (virtual_model_id) REFERENCES virtual_models(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vmr_parent ON virtual_model_routes(virtual_model_id)`,
		// regex_models is the pattern-based model alias table. priority
		// is ASC = highest first; ties break by insertion order which
		// is fine because admins can edit it. We pre-compile patterns
		// at router reload time so the request path never pays a
		// regexp.MustCompile cost.
		`CREATE TABLE IF NOT EXISTS regex_models (
			id           TEXT PRIMARY KEY,
			pattern      TEXT NOT NULL,
			priority     INTEGER NOT NULL DEFAULT 100,
			target_type  TEXT NOT NULL CHECK(target_type IN ('real','virtual')),
			target_model TEXT NOT NULL,
			provider     TEXT NOT NULL DEFAULT '',
			description  TEXT NOT NULL DEFAULT '',
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_regex_models_priority ON regex_models(priority ASC)`,

		// dlp_rules stores admin-defined content inspection rules.
		// Each rule carries a keyword or regex pattern that is matched
		// against request/response message content. The action column
		// determines what happens on a match: block (403), mask
		// (replace with ***), or log (allow but record).
		`CREATE TABLE IF NOT EXISTS dlp_rules (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			pattern       TEXT NOT NULL,
			pattern_type  TEXT NOT NULL CHECK(pattern_type IN ('keyword','regex')),
			scope         TEXT NOT NULL CHECK(scope IN ('request','response','both')),
			action        TEXT NOT NULL CHECK(action IN ('block','mask','log')),
			priority      INTEGER NOT NULL DEFAULT 100,
			model_pattern TEXT NOT NULL DEFAULT '',
			description   TEXT NOT NULL DEFAULT '',
			enabled       INTEGER NOT NULL DEFAULT 1,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_rules_priority ON dlp_rules(priority ASC)`,

		// dlp_violations is an append-only audit log of every DLP
		// match. rule_name is denormalised so entries remain readable
		// after the originating rule is deleted.
		`CREATE TABLE IF NOT EXISTS dlp_violations (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id       TEXT NOT NULL,
			rule_name     TEXT NOT NULL,
			key_id        TEXT NOT NULL DEFAULT '',
			model         TEXT NOT NULL DEFAULT '',
			direction     TEXT NOT NULL CHECK(direction IN ('request','response')),
			matched_text  TEXT NOT NULL DEFAULT '',
			action_taken  TEXT NOT NULL,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_ts ON dlp_violations(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_dlp_violations_rule ON dlp_violations(rule_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}

	// Safe schema upgrades for v2.5
	upgrades := []string{
		`ALTER TABLE admin_users ADD COLUMN nickname TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE admin_users ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE admin_users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range upgrades {
		_, _ = s.db.ExecContext(ctx, stmt) // ignore "duplicate column name" errors
	}
	return nil
}

// -- providers ----------------------------------------------------------

// ListProviders returns every provider row ordered by name. Disabled rows
// are included; callers that only want live providers should filter on
// Enabled.
func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, kind, api_key, base_url, api_version, region,
		       access_key, secret_key, session_token, deployments, models,
		       headers, timeout_sec, enabled, created_at, updated_at
		FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProvider loads one provider by name. Returns ErrNotFound when missing.
func (s *Store) GetProvider(ctx context.Context, name string) (Provider, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, kind, api_key, base_url, api_version, region,
		       access_key, secret_key, session_token, deployments, models,
		       headers, timeout_sec, enabled, created_at, updated_at
		FROM providers WHERE name = ?`, name)
	p, err := scanProvider(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	return p, err
}

// UpsertProvider inserts or replaces a provider row. The database populates
// CreatedAt on first insert and bumps UpdatedAt on every write.
func (s *Store) UpsertProvider(ctx context.Context, p Provider) error {
	if p.Name == "" {
		return errors.New("store: provider.name is required")
	}
	deployments, _ := json.Marshal(nilMap(p.Deployments))
	models, _ := json.Marshal(nilSlice(p.Models))
	headers, _ := json.Marshal(nilMap(p.Headers))
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO providers (
			name, kind, api_key, base_url, api_version, region,
			access_key, secret_key, session_token, deployments, models,
			headers, timeout_sec, enabled, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			kind          = excluded.kind,
			api_key       = excluded.api_key,
			base_url      = excluded.base_url,
			api_version   = excluded.api_version,
			region        = excluded.region,
			access_key    = excluded.access_key,
			secret_key    = excluded.secret_key,
			session_token = excluded.session_token,
			deployments   = excluded.deployments,
			models        = excluded.models,
			headers       = excluded.headers,
			timeout_sec   = excluded.timeout_sec,
			enabled       = excluded.enabled,
			updated_at    = CURRENT_TIMESTAMP`,
		p.Name, p.Kind, p.APIKey, p.BaseURL, p.APIVersion, p.Region,
		p.AccessKey, p.SecretKey, p.SessionToken, string(deployments),
		string(models), string(headers), p.TimeoutSec, boolToInt(p.Enabled))
	return err
}

// DeleteProvider removes a provider row. Dependent routes are left in
// place; the router reload will surface the dangling reference as a
// validation error so operators notice.
func (s *Store) DeleteProvider(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// -- routes -------------------------------------------------------------

// ListRoutes returns every route ordered by model.
func (s *Store) ListRoutes(ctx context.Context) ([]Route, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT model, provider, fallback, created_at, updated_at
		FROM routes ORDER BY model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Route
	for rows.Next() {
		r, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRoute loads one route by model.
func (s *Store) GetRoute(ctx context.Context, model string) (Route, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT model, provider, fallback, created_at, updated_at
		FROM routes WHERE model = ?`, model)
	r, err := scanRoute(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Route{}, ErrNotFound
	}
	return r, err
}

// UpsertRoute inserts or replaces a route row.
func (s *Store) UpsertRoute(ctx context.Context, r Route) error {
	if r.Model == "" || r.Provider == "" {
		return errors.New("store: route.model and route.provider are required")
	}
	fallback, _ := json.Marshal(nilSlice(r.Fallback))
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO routes (model, provider, fallback, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model) DO UPDATE SET
			provider   = excluded.provider,
			fallback   = excluded.fallback,
			updated_at = CURRENT_TIMESTAMP`,
		r.Model, r.Provider, string(fallback))
	return err
}

// DeleteRoute removes a route row.
func (s *Store) DeleteRoute(ctx context.Context, model string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE model = ?`, model)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// -- helpers ------------------------------------------------------------

// scanner is the subset of sql.Row/sql.Rows we need so scanProvider and
// scanRoute can serve both single-row and multi-row callers.
type scanner interface {
	Scan(dest ...any) error
}

func scanProvider(sc scanner) (Provider, error) {
	var (
		p                               Provider
		deployments, models, headers    string
		enabledInt                      int
		createdAt, updatedAt            time.Time
	)
	if err := sc.Scan(
		&p.Name, &p.Kind, &p.APIKey, &p.BaseURL, &p.APIVersion, &p.Region,
		&p.AccessKey, &p.SecretKey, &p.SessionToken, &deployments, &models,
		&headers, &p.TimeoutSec, &enabledInt, &createdAt, &updatedAt,
	); err != nil {
		return Provider{}, err
	}
	if deployments != "" {
		_ = json.Unmarshal([]byte(deployments), &p.Deployments)
	}
	if models != "" {
		_ = json.Unmarshal([]byte(models), &p.Models)
	}
	if headers != "" {
		_ = json.Unmarshal([]byte(headers), &p.Headers)
	}
	p.Enabled = enabledInt != 0
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return p, nil
}

func scanRoute(sc scanner) (Route, error) {
	var (
		r                    Route
		fallback             string
		createdAt, updatedAt time.Time
	)
	if err := sc.Scan(&r.Model, &r.Provider, &fallback, &createdAt, &updatedAt); err != nil {
		return Route{}, err
	}
	if fallback != "" {
		_ = json.Unmarshal([]byte(fallback), &r.Fallback)
	}
	r.CreatedAt = createdAt
	r.UpdatedAt = updatedAt
	return r, nil
}

func nilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func nilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
