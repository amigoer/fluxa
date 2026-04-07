package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromEnv_Defaults(t *testing.T) {
	// Clear any inherited vars so we observe the defaults.
	for _, k := range []string{
		"FLUXA_HOST", "FLUXA_PORT", "FLUXA_DB_PATH",
		"FLUXA_LOG_LEVEL", "FLUXA_LOG_FORMAT", "FLUXA_STORE_CONTENT",
		"FLUXA_READ_TIMEOUT", "FLUXA_WRITE_TIMEOUT", "FLUXA_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(k, "")
	}
	cfg := FromEnv()
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Path != "./fluxa.db" {
		t.Errorf("db path = %q, want ./fluxa.db", cfg.Database.Path)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("read timeout = %s, want 30s", cfg.Server.ReadTimeout)
	}
}

func TestFromEnv_Overrides(t *testing.T) {
	t.Setenv("FLUXA_HOST", "127.0.0.1")
	t.Setenv("FLUXA_PORT", "9090")
	t.Setenv("FLUXA_DB_PATH", "/tmp/f.db")
	t.Setenv("FLUXA_LOG_LEVEL", "debug")
	t.Setenv("FLUXA_READ_TIMEOUT", "15s")
	cfg := FromEnv()
	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 9090 {
		t.Errorf("listen = %s:%d", cfg.Server.Host, cfg.Server.Port)
	}
	if cfg.Database.Path != "/tmp/f.db" {
		t.Errorf("db path = %q", cfg.Database.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("log level = %q", cfg.Logging.Level)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("read timeout = %s", cfg.Server.ReadTimeout)
	}
}

func TestBundle_RoundTrip(t *testing.T) {
	providers := []ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk-test"},
	}
	routes := []RouteConfig{
		{Model: "gpt-4o", Provider: "openai", Fallback: []string{}},
	}
	raw, err := Marshal(providers, routes)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(b.Providers) != 1 || b.Providers[0].APIKey != "sk-test" {
		t.Errorf("providers round-trip mismatch: %+v", b.Providers)
	}
	if len(b.Routes) != 1 || b.Routes[0].Model != "gpt-4o" {
		t.Errorf("routes round-trip mismatch: %+v", b.Routes)
	}
}

func TestUnmarshal_EnvExpansion(t *testing.T) {
	t.Setenv("FLUXA_TEST_KEY", "sk-test")
	raw := []byte(`
providers:
  - name: openai
    api_key: ${FLUXA_TEST_KEY}
routes:
  - model: gpt-4o
    provider: openai
`)
	b, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.Providers[0].APIKey != "sk-test" {
		t.Errorf("api_key = %q, want sk-test", b.Providers[0].APIKey)
	}
	if b.Providers[0].Kind != "openai" {
		t.Errorf("kind should default to name, got %q", b.Providers[0].Kind)
	}
}

func TestUnmarshal_DefaultFromPlaceholder(t *testing.T) {
	raw := []byte(`
providers:
  - name: openai
    api_key: ${FLUXA_MISSING_VAR:-fallback-key}
routes: []
`)
	b, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.Providers[0].APIKey != "fallback-key" {
		t.Errorf("api_key = %q, want fallback-key", b.Providers[0].APIKey)
	}
}

func TestUnmarshal_UnknownProviderReference(t *testing.T) {
	raw := []byte(`
providers:
  - name: openai
    api_key: sk
routes:
  - model: gpt-4o
    provider: missing
`)
	if _, err := Unmarshal(raw); err == nil {
		t.Fatal("expected error for unknown provider reference")
	} else if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention missing provider, got %v", err)
	}
}

func TestUnmarshal_DuplicateProvider(t *testing.T) {
	raw := []byte(`
providers:
  - name: openai
    api_key: a
  - name: openai
    api_key: b
`)
	if _, err := Unmarshal(raw); err == nil {
		t.Fatal("expected duplicate provider error")
	}
}
