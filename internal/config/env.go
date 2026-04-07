// env.go — load the gateway's runtime configuration from environment
// variables. Every setting has a sensible default so `fluxa` binary
// can start with zero flags and zero files; operators only need to
// export the handful of env vars that apply to their deployment.
//
// Variables read (all optional unless noted):
//
//	FLUXA_HOST                — listen address            (default "0.0.0.0")
//	FLUXA_PORT                — listen port               (default 8080)
//	FLUXA_DB_PATH             — SQLite file path          (default "./fluxa.db")
//	FLUXA_LOG_LEVEL           — debug|info|warn|error     (default "info")
//	FLUXA_LOG_FORMAT          — json|text                 (default "json")
//	FLUXA_STORE_CONTENT       — persist request bodies    (default false)
//	FLUXA_READ_TIMEOUT        — HTTP read timeout         (default 30s)
//	FLUXA_WRITE_TIMEOUT       — HTTP write timeout        (default 5m)
//	FLUXA_SHUTDOWN_TIMEOUT    — graceful shutdown budget  (default 20s)
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
// on a fresh machine without ceremony, not to punish typos.
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
			Path: getEnv("FLUXA_DB_PATH", "./fluxa.db"),
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
