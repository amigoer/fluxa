package config

import (
	"os"
	"testing"
)

func TestParse_EnvExpansion(t *testing.T) {
	t.Setenv("FLUXA_TEST_KEY", "sk-test")
	raw := []byte(`
server:
  port: 9090
providers:
  - name: openai
    api_key: ${FLUXA_TEST_KEY}
routes:
  - model: gpt-4o
    provider: openai
`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Providers[0].APIKey != "sk-test" {
		t.Errorf("api_key = %q, want sk-test", cfg.Providers[0].APIKey)
	}
	if cfg.Providers[0].Kind != "openai" {
		t.Errorf("kind should default to name, got %q", cfg.Providers[0].Kind)
	}
}

func TestParse_DefaultFromPlaceholder(t *testing.T) {
	os.Unsetenv("FLUXA_MISSING_VAR")
	raw := []byte(`
providers:
  - name: openai
    api_key: ${FLUXA_MISSING_VAR:-fallback-key}
routes: []
`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Providers[0].APIKey != "fallback-key" {
		t.Errorf("api_key = %q, want fallback-key", cfg.Providers[0].APIKey)
	}
}

func TestValidate_UnknownProviderReference(t *testing.T) {
	raw := []byte(`
providers:
  - name: openai
    api_key: sk
routes:
  - model: gpt-4o
    provider: missing
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected error for unknown provider reference")
	}
}

func TestValidate_DuplicateProvider(t *testing.T) {
	raw := []byte(`
providers:
  - name: openai
    api_key: a
  - name: openai
    api_key: b
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected duplicate provider error")
	}
}
