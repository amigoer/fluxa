// Package store persists Fluxa's gateway state in Postgres: providers,
// routes, virtual keys, usage, admin accounts, model aliases, DLP rules
// and request logs. It replaces static configuration files entirely —
// operators mutate every one of these tables live through the admin API
// without restarting the process.
//
// Postgres is the only supported backend. The driver is pgx in its
// database/sql compatibility mode, which is pure Go, so the binary
// still builds with CGO_ENABLED=0.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/amigoer/fluxa/internal/config"
)

// ErrNotFound is returned when a lookup by primary key fails.
var ErrNotFound = errors.New("store: not found")

// migrationLockKey is the advisory lock every process takes before
// running migrations. Several replicas booting at once would otherwise
// race inside CREATE TABLE IF NOT EXISTS, which is not atomic against
// a concurrent create in Postgres and fails with a duplicate pg_type
// error. The value is arbitrary but must stay stable across releases.
const migrationLockKey int64 = 0x666C7578_61 // "flux" + 'a'

// Store wraps a *sql.DB handle and exposes typed CRUD helpers for every
// gateway table.
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

// Open connects to Postgres, verifies the connection, and applies the
// schema migrations. The returned Store owns its pool; call Close when
// the process shuts down.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", cfg.Redacted(), err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: connect %s: %w", cfg.Redacted(), err)
	}
	s := &Store{db: db}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// New wraps an already-open Postgres handle. It exists for callers that
// manage the pool themselves (tests, embedding Fluxa in a larger app);
// they are responsible for closing the handle and for calling Migrate.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for advanced callers (tests, admin tooling).
func (s *Store) DB() *sql.DB { return s.db }

// Migrate creates the schema on an empty database. Every statement is
// idempotent, so running it against an existing database is a no-op and
// a restart never needs a separate migration step. Concurrent callers
// serialise on a session-level advisory lock.
func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	defer conn.Close()

	// The lock is taken on this specific connection and released on the
	// same one, so a crash mid-migration frees it when the session dies.
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("store: migrate: acquire lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	for _, stmt := range schemaStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

// schemaStatements is the full schema in dependency order. JSON-shaped
// columns are JSONB so operators can query them directly in psql; the
// Go layer still marshals and unmarshals them as opaque documents.
var schemaStatements = []string{
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
		deployments     JSONB NOT NULL DEFAULT '{}'::jsonb,
		models          JSONB NOT NULL DEFAULT '[]'::jsonb,
		headers         JSONB NOT NULL DEFAULT '{}'::jsonb,
		timeout_sec     INTEGER NOT NULL DEFAULT 0,
		enabled         BOOLEAN NOT NULL DEFAULT TRUE,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS routes (
		model           TEXT PRIMARY KEY,
		provider        TEXT NOT NULL,
		fallback        JSONB NOT NULL DEFAULT '[]'::jsonb,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider) REFERENCES providers(name) ON UPDATE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_routes_provider ON routes(provider)`,
	`CREATE TABLE IF NOT EXISTS virtual_keys (
		id                    TEXT PRIMARY KEY,
		name                  TEXT NOT NULL,
		description           TEXT NOT NULL DEFAULT '',
		models                JSONB NOT NULL DEFAULT '[]'::jsonb,
		ip_allowlist          JSONB NOT NULL DEFAULT '[]'::jsonb,
		budget_tokens_daily   BIGINT NOT NULL DEFAULT 0,
		budget_tokens_monthly BIGINT NOT NULL DEFAULT 0,
		budget_usd_daily      DOUBLE PRECISION NOT NULL DEFAULT 0,
		budget_usd_monthly    DOUBLE PRECISION NOT NULL DEFAULT 0,
		rpm_limit             INTEGER NOT NULL DEFAULT 0,
		enabled               BOOLEAN NOT NULL DEFAULT TRUE,
		expires_at            TIMESTAMPTZ,
		created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS usage_records (
		id                 BIGSERIAL PRIMARY KEY,
		virtual_key_id     TEXT NOT NULL,
		ts                 TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		model              TEXT NOT NULL,
		provider           TEXT NOT NULL,
		prompt_tokens      INTEGER NOT NULL DEFAULT 0,
		completion_tokens  INTEGER NOT NULL DEFAULT 0,
		total_tokens       INTEGER NOT NULL DEFAULT 0,
		cost_usd           DOUBLE PRECISION NOT NULL DEFAULT 0,
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
		id            BIGSERIAL PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		nickname      TEXT NOT NULL DEFAULT '',
		email         TEXT NOT NULL DEFAULT '',
		avatar_url    TEXT NOT NULL DEFAULT '',
		created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS admin_sessions (
		token       TEXT PRIMARY KEY,
		user_id     BIGINT NOT NULL,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at  TIMESTAMPTZ NOT NULL,
		FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON admin_sessions(expires_at)`,
	// virtual_models is the "user-facing model name" alias table. A
	// virtual model fans out to one or more real (or virtual) targets
	// with weighted traffic splitting; the resolver in
	// internal/router/model_resolver.go evaluates the chain at request
	// time. ON DELETE CASCADE on the child table keeps an admin
	// "delete virtual model" call from leaving orphaned route rows.
	`CREATE TABLE IF NOT EXISTS virtual_models (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		enabled     BOOLEAN NOT NULL DEFAULT TRUE,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS virtual_model_routes (
		id               TEXT PRIMARY KEY,
		virtual_model_id TEXT NOT NULL,
		weight           INTEGER NOT NULL CHECK (weight > 0),
		target_type      TEXT NOT NULL CHECK (target_type IN ('real','virtual')),
		target_model     TEXT NOT NULL,
		provider         TEXT NOT NULL DEFAULT '',
		enabled          BOOLEAN NOT NULL DEFAULT TRUE,
		position         INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (virtual_model_id) REFERENCES virtual_models(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_vmr_parent ON virtual_model_routes(virtual_model_id)`,
	// regex_models is the pattern-based model alias table. priority is
	// ASC = highest first; ties break by created_at. Patterns are
	// pre-compiled at router reload time so the request path never pays
	// a regexp.Compile cost.
	`CREATE TABLE IF NOT EXISTS regex_models (
		id           TEXT PRIMARY KEY,
		pattern      TEXT NOT NULL,
		priority     INTEGER NOT NULL DEFAULT 100,
		target_type  TEXT NOT NULL CHECK (target_type IN ('real','virtual')),
		target_model TEXT NOT NULL,
		provider     TEXT NOT NULL DEFAULT '',
		description  TEXT NOT NULL DEFAULT '',
		enabled      BOOLEAN NOT NULL DEFAULT TRUE,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_regex_models_priority ON regex_models(priority ASC)`,
	// dlp_rules stores admin-defined content inspection rules. Each rule
	// carries a keyword or regex pattern matched against request or
	// response content. The action column decides what happens on a
	// match: block (403), mask (replace with ***), or log (allow but
	// record).
	`CREATE TABLE IF NOT EXISTS dlp_rules (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		pattern       TEXT NOT NULL,
		pattern_type  TEXT NOT NULL CHECK (pattern_type IN ('keyword','regex')),
		scope         TEXT NOT NULL CHECK (scope IN ('request','response','both')),
		action        TEXT NOT NULL CHECK (action IN ('block','mask','log')),
		priority      INTEGER NOT NULL DEFAULT 100,
		model_pattern TEXT NOT NULL DEFAULT '',
		description   TEXT NOT NULL DEFAULT '',
		enabled       BOOLEAN NOT NULL DEFAULT TRUE,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dlp_rules_priority ON dlp_rules(priority ASC)`,
	// dlp_violations is an append-only audit log of every DLP match.
	// rule_name is denormalised so entries remain readable after the
	// originating rule is deleted.
	`CREATE TABLE IF NOT EXISTS dlp_violations (
		id            BIGSERIAL PRIMARY KEY,
		rule_id       TEXT NOT NULL,
		rule_name     TEXT NOT NULL,
		key_id        TEXT NOT NULL DEFAULT '',
		model         TEXT NOT NULL DEFAULT '',
		direction     TEXT NOT NULL CHECK (direction IN ('request','response')),
		matched_text  TEXT NOT NULL DEFAULT '',
		action_taken  TEXT NOT NULL,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dlp_violations_ts ON dlp_violations(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_dlp_violations_rule ON dlp_violations(rule_id)`,
	// request_logs is the per-call raw log: one row per incoming
	// /v1/chat/completions or /v1/messages request, whether it
	// succeeded, failed, or was blocked by DLP. request_body and
	// response_body hold the full payloads so operators can reproduce
	// calls and audit what data left the network. usage_records remains
	// the aggregation table (budgets, dashboards); request_logs is the
	// raw stream and is expected to be pruned by a retention job.
	`CREATE TABLE IF NOT EXISTS request_logs (
		id                TEXT PRIMARY KEY,
		virtual_key_id    TEXT NOT NULL DEFAULT '',
		started_at        TIMESTAMPTZ NOT NULL,
		first_byte_at     TIMESTAMPTZ,
		completed_at      TIMESTAMPTZ NOT NULL,
		endpoint          TEXT NOT NULL DEFAULT '',
		method            TEXT NOT NULL DEFAULT 'POST',
		model_requested   TEXT NOT NULL DEFAULT '',
		model_resolved    TEXT NOT NULL DEFAULT '',
		provider          TEXT NOT NULL DEFAULT '',
		is_stream         BOOLEAN NOT NULL DEFAULT FALSE,
		cache_hit         BOOLEAN NOT NULL DEFAULT FALSE,
		status_code       INTEGER NOT NULL DEFAULT 0,
		error             TEXT NOT NULL DEFAULT '',
		prompt_tokens     INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens      INTEGER NOT NULL DEFAULT 0,
		cost_usd          DOUBLE PRECISION NOT NULL DEFAULT 0,
		latency_ms        INTEGER NOT NULL DEFAULT 0,
		ttft_ms           INTEGER NOT NULL DEFAULT 0,
		request_body      TEXT NOT NULL DEFAULT '',
		response_body     TEXT NOT NULL DEFAULT '',
		client_ip         TEXT NOT NULL DEFAULT '',
		user_agent        TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_request_logs_started ON request_logs(started_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_request_logs_key ON request_logs(virtual_key_id, started_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_request_logs_status ON request_logs(status_code)`,
	`CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs(model_resolved)`,
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
		FROM providers WHERE name = $1`, name)
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
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, CURRENT_TIMESTAMP)
		ON CONFLICT (name) DO UPDATE SET
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
		string(models), string(headers), p.TimeoutSec, p.Enabled)
	return err
}

// DeleteProvider removes a provider row. Dependent routes are left in
// place; the router reload will surface the dangling reference as a
// validation error so operators notice.
func (s *Store) DeleteProvider(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE name = $1`, name)
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
		FROM routes WHERE model = $1`, model)
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
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (model) DO UPDATE SET
			provider   = excluded.provider,
			fallback   = excluded.fallback,
			updated_at = CURRENT_TIMESTAMP`,
		r.Model, r.Provider, string(fallback))
	return err
}

// DeleteRoute removes a route row.
func (s *Store) DeleteRoute(ctx context.Context, model string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE model = $1`, model)
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
		p                            Provider
		deployments, models, headers []byte
		createdAt, updatedAt         time.Time
	)
	if err := sc.Scan(
		&p.Name, &p.Kind, &p.APIKey, &p.BaseURL, &p.APIVersion, &p.Region,
		&p.AccessKey, &p.SecretKey, &p.SessionToken, &deployments, &models,
		&headers, &p.TimeoutSec, &p.Enabled, &createdAt, &updatedAt,
	); err != nil {
		return Provider{}, err
	}
	if len(deployments) > 0 {
		_ = json.Unmarshal(deployments, &p.Deployments)
	}
	if len(models) > 0 {
		_ = json.Unmarshal(models, &p.Models)
	}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &p.Headers)
	}
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return p, nil
}

func scanRoute(sc scanner) (Route, error) {
	var (
		r                    Route
		fallback             []byte
		createdAt, updatedAt time.Time
	)
	if err := sc.Scan(&r.Model, &r.Provider, &fallback, &createdAt, &updatedAt); err != nil {
		return Route{}, err
	}
	if len(fallback) > 0 {
		_ = json.Unmarshal(fallback, &r.Fallback)
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
