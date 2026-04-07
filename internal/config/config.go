// Package config holds the Fluxa gateway's runtime configuration
// schema and its YAML import/export bundle.
//
// Starting with v2.1 Fluxa boots from environment variables only.
// Providers and routes live exclusively in the SQLite store and are
// mutated at runtime through the /admin REST API. The YAML format
// that earlier versions used for bootstrap is now exposed through
// the /admin/config/export and /admin/config/import endpoints so
// operators can still snapshot and restore the full gateway state
// in one human-readable file.
//
// This file declares the shared type vocabulary used by env.go,
// yaml.go, store/bootstrap.go, router.Reload, and the admin key
// wiring. Keeping them in one package lets the store continue to
// speak in ProviderConfig / RouteConfig DTOs without dragging a
// YAML parser into the data plane.

package config

import "time"

// Runtime bundles every setting the gateway process reads at
// startup. It is populated by FromEnv() and never serialised — the
// YAML bundle type (see yaml.go) is what the import/export admin
// endpoints round-trip through.
type Runtime struct {
	Server   ServerConfig
	Database DatabaseConfig
	Logging  LoggingConfig
}

// ServerConfig controls the HTTP listener. Admin authentication has
// moved out of the static config: operators now sign in with a
// username + password against the admin_users table, so there is no
// master-key field here any more.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds storage configuration. SQLite is the default
// and only supported backend in v2.x; Postgres support is planned.
type DatabaseConfig struct {
	Path string
}

// LoggingConfig controls the structured logger output.
type LoggingConfig struct {
	Level        string
	Format       string
	StoreContent bool
}

// ProviderConfig is the DTO for an upstream model provider. It is
// used both by the admin import/export YAML schema (via its `yaml`
// tags) and as the in-memory handoff type between the store and
// the router.
type ProviderConfig struct {
	Name         string            `yaml:"name"`
	Kind         string            `yaml:"kind,omitempty"`
	APIKey       string            `yaml:"api_key,omitempty"`
	BaseURL      string            `yaml:"base_url,omitempty"`
	APIVersion   string            `yaml:"api_version,omitempty"`
	Region       string            `yaml:"region,omitempty"`
	AccessKey    string            `yaml:"access_key,omitempty"`
	SecretKey    string            `yaml:"secret_key,omitempty"`
	SessionToken string            `yaml:"session_token,omitempty"`
	Deployments  map[string]string `yaml:"deployments,omitempty"`
	Models       []string          `yaml:"models,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	Timeout      time.Duration     `yaml:"timeout,omitempty"`
}

// RouteConfig maps a model identifier to a primary provider plus
// an ordered list of fallback providers used when the primary
// fails.
type RouteConfig struct {
	Model    string   `yaml:"model"`
	Provider string   `yaml:"provider"`
	Fallback []string `yaml:"fallback,omitempty"`
}
