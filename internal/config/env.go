// env.go — load the gateway's runtime configuration from environment
// variables. Everything except the database has a sensible default, so
// operators only need to point Fluxa at a Postgres instance and export
// the handful of vars that apply to their deployment.
//
// Variables read (all optional unless noted):
//
//	FLUXA_HOST                — listen address            (default "0.0.0.0")
//	FLUXA_PORT                — listen port               (default 8080)
//	FLUXA_LOG_LEVEL           — debug|info|warn|error     (default "info")
//	FLUXA_LOG_FORMAT          — json|text                 (default "json")
//	FLUXA_STORE_CONTENT       — persist request bodies    (default false)
//	FLUXA_READ_TIMEOUT        — HTTP read timeout         (default 30s)
//	FLUXA_WRITE_TIMEOUT       — HTTP write timeout        (default 5m)
//	FLUXA_SHUTDOWN_TIMEOUT    — graceful shutdown budget  (default 20s)
//
// Database — either supply a full URL:
//
//	FLUXA_DATABASE_URL        — postgres://user:pass@host:5432/db?sslmode=disable
//
// or the discrete parts (ignored entirely when the URL is set):
//
//	FLUXA_DB_HOST             — Postgres host             (default "localhost")
//	FLUXA_DB_PORT             — Postgres port             (default 5432)
//	FLUXA_DB_USER             — Postgres role             (default "fluxa")
//	FLUXA_DB_PASSWORD         — role password             (default "")
//	FLUXA_DB_NAME             — database name             (default "fluxa")
//	FLUXA_DB_SSLMODE          — libpq sslmode             (default "disable")
//
// Connection pool tuning:
//
//	FLUXA_DB_MAX_OPEN_CONNS   — max open connections      (default 25)
//	FLUXA_DB_MAX_IDLE_CONNS   — max idle connections      (default 5)
//	FLUXA_DB_CONN_MAX_LIFETIME — connection max age       (default 1h)
//	FLUXA_DB_CONN_MAX_IDLETIME — idle connection max age  (default 10m)
//
// Bootstrap-only env vars consumed by main.go (not part of Runtime):
//
//	FLUXA_BOOTSTRAP_USER      — first-run admin username  (default "admin")
//	FLUXA_BOOTSTRAP_PASSWORD  — first-run admin password  (default "admin")

package config

import (
	"os"
	"strconv"
	"time"
)

// FromEnv returns a Runtime populated from the process environment.
// Invalid values silently fall back to defaults: the goal is to boot
// without ceremony, not to punish typos. Connection settings that
// cannot work at all are caught by DatabaseConfig.Validate instead.
func FromEnv() Runtime {
	return Runtime{
		Server: ServerConfig{
			Host:            getEnv("FLUXA_HOST", "0.0.0.0"),
			Port:            getEnvInt("FLUXA_PORT", 8080),
			ReadTimeout:     getEnvDuration("FLUXA_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("FLUXA_WRITE_TIMEOUT", 5*time.Minute),
			ShutdownTimeout: getEnvDuration("FLUXA_SHUTDOWN_TIMEOUT", 20*time.Second),
		},
		Database: DatabaseConfig{
			URL:             os.Getenv("FLUXA_DATABASE_URL"),
			Host:            getEnv("FLUXA_DB_HOST", "localhost"),
			Port:            getEnvInt("FLUXA_DB_PORT", 5432),
			User:            getEnv("FLUXA_DB_USER", "fluxa"),
			Password:        os.Getenv("FLUXA_DB_PASSWORD"),
			Name:            getEnv("FLUXA_DB_NAME", "fluxa"),
			SSLMode:         getEnv("FLUXA_DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("FLUXA_DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("FLUXA_DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("FLUXA_DB_CONN_MAX_LIFETIME", time.Hour),
			ConnMaxIdleTime: getEnvDuration("FLUXA_DB_CONN_MAX_IDLETIME", 10*time.Minute),
		},
		Logging: LoggingConfig{
			Level:        getEnv("FLUXA_LOG_LEVEL", "info"),
			Format:       getEnv("FLUXA_LOG_FORMAT", "json"),
			StoreContent: getEnvBool("FLUXA_STORE_CONTENT", false),
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
