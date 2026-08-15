// Package config loads process-level settings from the environment.
//
// Business configuration (identity provider credentials, notify channel
// credentials, DLP rules, ...) lives in the database and is managed by
// admins through the UI. This package only covers what the process needs
// before it can even reach the database: where to listen and how to
// connect.
package config

import (
	"fmt"
	"os"
)

// Config holds the settings the server needs at startup.
type Config struct {
	// ListenAddr is the address the HTTP server binds to, e.g. ":8080".
	ListenAddr string

	// DatabaseURL is a standard PostgreSQL connection string.
	DatabaseURL string

	// SessionCookieName is the cookie the server uses to carry the
	// opaque session token described in DESIGN.md 7.1 (server-side
	// session stored in Postgres, no JWT, no Redis).
	SessionCookieName string

	// SessionCookieSecure controls the Secure flag on the session
	// cookie. It should be true in any real deployment (HTTPS) and can
	// be turned off for local HTTP development.
	SessionCookieSecure bool

	// BaseURL is this deployment's own externally-reachable origin, used
	// to build OAuth redirect URIs (e.g. the Feishu callback).
	BaseURL string
}

// Load reads configuration from environment variables, applying sane
// defaults for local development.
func Load() (Config, error) {
	cfg := Config{
		ListenAddr:          getEnv("FLUXA_LISTEN_ADDR", ":8080"),
		DatabaseURL:         getEnv("FLUXA_DATABASE_URL", ""),
		SessionCookieName:   getEnv("FLUXA_SESSION_COOKIE", "fluxa_session"),
		SessionCookieSecure: getEnv("FLUXA_SESSION_COOKIE_SECURE", "true") == "true",
		BaseURL:             getEnv("FLUXA_BASE_URL", "http://localhost:8080"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: FLUXA_DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
