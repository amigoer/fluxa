// Package config holds the Fluxa gateway's runtime configuration
// schema.
//
// Fluxa boots from environment variables only. There is no config
// file: everything the process needs at startup is a listen address,
// a Postgres connection, and logging preferences. Providers, routes,
// virtual models and every other piece of gateway state live in
// Postgres and are mutated at runtime through the /admin REST API.
//
// This file declares the shared type vocabulary used by env.go,
// store/bootstrap.go, router.Reload, and the admin key wiring.
// Keeping them in one package lets the store speak in ProviderConfig
// / RouteConfig DTOs without importing the router.

package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

// Runtime bundles every setting the gateway process reads at startup.
// It is populated by FromEnv() and never serialised.
type Runtime struct {
	Server   ServerConfig
	Database DatabaseConfig
	Logging  LoggingConfig
}

// ServerConfig controls the HTTP listener. Admin authentication is not
// configured here: operators sign in with a username + password against
// the admin_users table.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig describes the Postgres connection. Postgres is the
// only supported backend.
//
// URL, when set, is used verbatim and every discrete field below is
// ignored — that is the form platforms like Fly, Render and Heroku
// inject as DATABASE_URL. Otherwise the discrete fields are assembled
// into a DSN, which is friendlier for docker-compose deployments where
// each part comes from its own environment variable.
type DatabaseConfig struct {
	URL      string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string

	// Pool tuning. Postgres connections are expensive server-side, so
	// the gateway keeps a bounded pool rather than one connection per
	// in-flight request.
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN returns the connection string handed to the pgx driver. When URL
// is set it wins untouched; otherwise the discrete fields are rendered
// as a postgres:// URL with the password escaped so exotic characters
// survive the round trip.
func (d DatabaseConfig) DSN() string {
	if d.URL != "" {
		return d.URL
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   "/" + d.Name,
	}
	if d.User != "" {
		if d.Password != "" {
			u.User = url.UserPassword(d.User, d.Password)
		} else {
			u.User = url.User(d.User)
		}
	}
	if d.SSLMode != "" {
		u.RawQuery = url.Values{"sslmode": []string{d.SSLMode}}.Encode()
	}
	return u.String()
}

// Redacted returns the DSN with any password replaced by "xxxxx". Use
// it for log lines — a connection string is the first thing that ends
// up in a pasted stack trace.
func (d DatabaseConfig) Redacted() string {
	u, err := url.Parse(d.DSN())
	if err != nil {
		return "postgres://<unparseable dsn>"
	}
	return u.Redacted()
}

// Validate reports whether the connection settings are complete enough
// to attempt a dial. It exists so a misconfigured deployment fails at
// boot with a readable message instead of a driver-level error.
func (d DatabaseConfig) Validate() error {
	if d.URL != "" {
		u, err := url.Parse(d.URL)
		if err != nil {
			return fmt.Errorf("FLUXA_DATABASE_URL is not a valid URL: %w", err)
		}
		if u.Scheme != "postgres" && u.Scheme != "postgresql" {
			return fmt.Errorf("FLUXA_DATABASE_URL scheme %q is not postgres", u.Scheme)
		}
		return nil
	}
	if d.Host == "" {
		return fmt.Errorf("database host is required (set FLUXA_DATABASE_URL or FLUXA_DB_HOST)")
	}
	if d.Port <= 0 || d.Port > 65535 {
		return fmt.Errorf("database port %d is out of range", d.Port)
	}
	if d.Name == "" {
		return fmt.Errorf("database name is required (set FLUXA_DATABASE_URL or FLUXA_DB_NAME)")
	}
	if d.User == "" {
		return fmt.Errorf("database user is required (set FLUXA_DATABASE_URL or FLUXA_DB_USER)")
	}
	return nil
}

// LoggingConfig controls the structured logger output.
type LoggingConfig struct {
	Level        string
	Format       string
	StoreContent bool
}

// ProviderConfig is the in-memory DTO for an upstream model provider,
// used as the handoff type between the store and the router.
type ProviderConfig struct {
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
	Timeout      time.Duration
}

// RouteConfig maps a model identifier to a primary provider plus an
// ordered list of fallback providers used when the primary fails. The
// "stable alias" use case (one caller-facing name that rolls across
// multiple real model versions) is owned by virtual_models, not by
// RouteConfig.
type RouteConfig struct {
	Model    string
	Provider string
	Fallback []string
}
