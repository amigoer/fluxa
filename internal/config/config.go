// Package config loads and validates the Fluxa gateway configuration from a
// YAML file. The loader supports ${VAR} / ${VAR:-default} environment variable
// expansion so secrets can live outside the file and the same config can be
// used across environments.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level gateway configuration.
type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Database  DatabaseConfig   `yaml:"database"`
	Providers []ProviderConfig `yaml:"providers"`
	Routes    []RouteConfig    `yaml:"routes"`
	Logging   LoggingConfig    `yaml:"logging"`
}

// ServerConfig controls the HTTP listener and admin authentication.
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	MasterKey       string        `yaml:"master_key"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig holds database configuration. SQLite is the default and only
// supported backend in v1.0; Postgres support is planned for v6.0.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ProviderConfig is an upstream model provider configuration.
type ProviderConfig struct {
	// Name is the unique provider identifier referenced by routes.
	Name string `yaml:"name"`
	// Kind selects the adapter implementation. Supported values in v1.0:
	// "openai", "anthropic", "deepseek", "qwen", "ollama". When empty it
	// defaults to Name so that the simple single-provider case works with no
	// extra configuration.
	Kind    string            `yaml:"kind"`
	APIKey  string            `yaml:"api_key"`
	BaseURL string            `yaml:"base_url"`
	Timeout time.Duration     `yaml:"timeout"`
	Headers map[string]string `yaml:"headers"`

	// Models is the list of model identifiers this provider advertises via
	// GET /v1/models. When empty the provider serves only the models named
	// in the routes section.
	Models []string `yaml:"models"`

	// Region is consumed by cloud providers that require a regional
	// endpoint (AWS Bedrock, Azure). Ignored by the OpenAI-compatible kinds.
	Region string `yaml:"region"`

	// APIVersion is consumed by adapters that pin a specific API revision
	// (Azure OpenAI uses e.g. "2024-02-15-preview", Anthropic uses
	// "2023-06-01"). Ignored when empty.
	APIVersion string `yaml:"api_version"`

	// Deployments maps canonical model identifiers to provider-specific
	// deployment names. Azure OpenAI requires a per-deployment URL, so this
	// is how operators bridge e.g. "gpt-4o" to their Azure deployment name.
	Deployments map[string]string `yaml:"deployments"`

	// AccessKey / SecretKey / SessionToken carry AWS credentials for the
	// Bedrock adapter. They are plain strings instead of a nested struct so
	// that ${VAR} interpolation works on each field.
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	SessionToken string `yaml:"session_token"`
}

// RouteConfig maps a model identifier to a primary provider plus an ordered
// list of fallback providers used when the primary fails.
type RouteConfig struct {
	Model    string   `yaml:"model"`
	Provider string   `yaml:"provider"`
	Fallback []string `yaml:"fallback"`
}

// LoggingConfig controls the structured logger output.
type LoggingConfig struct {
	Level        string `yaml:"level"`
	Format       string `yaml:"format"`
	StoreContent bool   `yaml:"store_content"`
}

// Default returns a Config populated with sensible defaults. Callers overlay
// values from the YAML file on top of these defaults.
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    5 * time.Minute,
			ShutdownTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{Path: "./fluxa.db"},
		Logging:  LoggingConfig{Level: "info", Format: "json"},
	}
}

// Load reads the YAML file at path, expands environment variables and returns
// a validated Config.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return Parse(raw)
}

// Parse decodes YAML bytes into a Config. Exposed so tests and tools can feed
// configs from memory.
func Parse(raw []byte) (Config, error) {
	expanded := expandEnv(string(raw))
	cfg := Default()
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse yaml: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyDefaults fills in per-section defaults that depend on other fields.
func (c *Config) applyDefaults() {
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.Kind == "" {
			p.Kind = p.Name
		}
		if p.Timeout == 0 {
			p.Timeout = 5 * time.Minute
		}
	}
}

// Validate ensures the configuration has the minimum fields required to start
// the gateway. It returns the first error encountered so operators can fix
// problems incrementally.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d out of range", c.Server.Port)
	}
	// providers and routes may be empty when they are sourced from the
	// database instead of the YAML file. The router layer enforces the
	// "at least one provider" invariant at reload time.
	seen := make(map[string]struct{}, len(c.Providers))
	for _, p := range c.Providers {
		if p.Name == "" {
			return errors.New("provider.name is required")
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("duplicate provider name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
		switch p.Kind {
		case "ollama":
			// No credentials required.
		case "bedrock":
			if p.AccessKey == "" || p.SecretKey == "" || p.Region == "" {
				return fmt.Errorf("provider %q: access_key, secret_key and region are required", p.Name)
			}
		default:
			if p.APIKey == "" {
				return fmt.Errorf("provider %q: api_key is required", p.Name)
			}
		}
	}
	for _, r := range c.Routes {
		if r.Model == "" {
			return errors.New("route.model is required")
		}
		if _, ok := seen[r.Provider]; !ok {
			return fmt.Errorf("route %q references unknown provider %q", r.Model, r.Provider)
		}
		for _, fb := range r.Fallback {
			if _, ok := seen[fb]; !ok {
				return fmt.Errorf("route %q fallback references unknown provider %q", r.Model, fb)
			}
		}
	}
	return nil
}

// envPattern matches ${VAR} and ${VAR:-default} style placeholders.
var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandEnv replaces ${VAR} placeholders inside the YAML source with their
// environment values. Unknown variables expand to the empty string unless a
// ":-default" clause is supplied.
func expandEnv(src string) string {
	return envPattern.ReplaceAllStringFunc(src, func(match string) string {
		groups := envPattern.FindStringSubmatch(match)
		if v, ok := os.LookupEnv(groups[1]); ok {
			return v
		}
		return groups[2]
	})
}
