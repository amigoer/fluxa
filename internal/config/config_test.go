package config

import (
	"strings"
	"testing"
	"time"
)

// dbEnvVars are every database-related variable FromEnv reads. Tests
// clear them wholesale so a developer's exported FLUXA_DATABASE_URL
// cannot leak into an assertion about defaults.
var dbEnvVars = []string{
	"FLUXA_DATABASE_URL", "FLUXA_DB_HOST", "FLUXA_DB_PORT", "FLUXA_DB_USER",
	"FLUXA_DB_PASSWORD", "FLUXA_DB_NAME", "FLUXA_DB_SSLMODE",
	"FLUXA_DB_MAX_OPEN_CONNS", "FLUXA_DB_MAX_IDLE_CONNS",
	"FLUXA_DB_CONN_MAX_LIFETIME", "FLUXA_DB_CONN_MAX_IDLETIME",
}

func TestFromEnv_Defaults(t *testing.T) {
	// Clear any inherited vars so we observe the defaults.
	for _, k := range append([]string{
		"FLUXA_HOST", "FLUXA_PORT",
		"FLUXA_LOG_LEVEL", "FLUXA_LOG_FORMAT", "FLUXA_STORE_CONTENT",
		"FLUXA_READ_TIMEOUT", "FLUXA_WRITE_TIMEOUT", "FLUXA_SHUTDOWN_TIMEOUT",
	}, dbEnvVars...) {
		t.Setenv(k, "")
	}
	cfg := FromEnv()
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
	if got, want := cfg.Database.DSN(), "postgres://fluxa@localhost:5432/fluxa?sslmode=disable"; got != want {
		t.Errorf("dsn = %q, want %q", got, want)
	}
	if cfg.Database.MaxOpenConns != 25 || cfg.Database.MaxIdleConns != 5 {
		t.Errorf("pool = %d/%d, want 25/5", cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != time.Hour {
		t.Errorf("conn max lifetime = %s, want 1h", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("read timeout = %s, want 30s", cfg.Server.ReadTimeout)
	}
}

func TestFromEnv_Overrides(t *testing.T) {
	t.Setenv("FLUXA_HOST", "127.0.0.1")
	t.Setenv("FLUXA_PORT", "9090")
	t.Setenv("FLUXA_DB_HOST", "db.internal")
	t.Setenv("FLUXA_DB_PORT", "6432")
	t.Setenv("FLUXA_DB_USER", "gateway")
	t.Setenv("FLUXA_DB_PASSWORD", "s3cret")
	t.Setenv("FLUXA_DB_NAME", "fluxa_prod")
	t.Setenv("FLUXA_DB_SSLMODE", "require")
	t.Setenv("FLUXA_LOG_LEVEL", "debug")
	t.Setenv("FLUXA_READ_TIMEOUT", "15s")
	cfg := FromEnv()
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 9090 {
		t.Errorf("listen = %s:%d", cfg.Server.Host, cfg.Server.Port)
	}
	if got, want := cfg.Database.DSN(), "postgres://gateway:s3cret@db.internal:6432/fluxa_prod?sslmode=require"; got != want {
		t.Errorf("dsn = %q, want %q", got, want)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("log level = %q", cfg.Logging.Level)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("read timeout = %s", cfg.Server.ReadTimeout)
	}
}

// A supplied URL is passed through untouched and the discrete fields
// are ignored — platforms that inject DATABASE_URL expect exactly that.
func TestFromEnv_DatabaseURLWins(t *testing.T) {
	t.Setenv("FLUXA_DATABASE_URL", "postgres://u:p@example.com:5555/db?sslmode=verify-full")
	t.Setenv("FLUXA_DB_HOST", "ignored")
	t.Setenv("FLUXA_DB_NAME", "ignored")
	cfg := FromEnv()
	if got, want := cfg.Database.DSN(), "postgres://u:p@example.com:5555/db?sslmode=verify-full"; got != want {
		t.Errorf("dsn = %q, want %q", got, want)
	}
}

func TestDatabaseConfig_DSNEscapesPassword(t *testing.T) {
	d := DatabaseConfig{Host: "localhost", Port: 5432, User: "fluxa", Password: "p@ss/word", Name: "fluxa", SSLMode: "disable"}
	dsn := d.DSN()
	if strings.Contains(dsn, "p@ss/word") {
		t.Errorf("password not escaped in dsn: %q", dsn)
	}
	if !strings.Contains(dsn, "p%40ss%2Fword") {
		t.Errorf("dsn = %q, want percent-escaped password", dsn)
	}
}

func TestDatabaseConfig_Redacted(t *testing.T) {
	d := DatabaseConfig{Host: "localhost", Port: 5432, User: "fluxa", Password: "hunter2", Name: "fluxa"}
	if got := d.Redacted(); strings.Contains(got, "hunter2") {
		t.Errorf("redacted dsn leaks password: %q", got)
	}
}

func TestDatabaseConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DatabaseConfig
		wantErr string
	}{
		{
			name: "discrete fields ok",
			cfg:  DatabaseConfig{Host: "localhost", Port: 5432, User: "fluxa", Name: "fluxa"},
		},
		{
			name: "url ok",
			cfg:  DatabaseConfig{URL: "postgres://u@h:5432/db"},
		},
		{
			name:    "url with wrong scheme",
			cfg:     DatabaseConfig{URL: "mysql://u@h:3306/db"},
			wantErr: "not postgres",
		},
		{
			name:    "missing host",
			cfg:     DatabaseConfig{Port: 5432, User: "fluxa", Name: "fluxa"},
			wantErr: "host is required",
		},
		{
			name:    "missing name",
			cfg:     DatabaseConfig{Host: "localhost", Port: 5432, User: "fluxa"},
			wantErr: "name is required",
		},
		{
			name:    "missing user",
			cfg:     DatabaseConfig{Host: "localhost", Port: 5432, Name: "fluxa"},
			wantErr: "user is required",
		},
		{
			name:    "port out of range",
			cfg:     DatabaseConfig{Host: "localhost", Port: 70000, User: "fluxa", Name: "fluxa"},
			wantErr: "out of range",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Validate() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}
