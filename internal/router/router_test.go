package router

import (
	"testing"

	"github.com/amigoer/fluxa/internal/config"
)

// buildForTest is a tiny helper that spins up a Router from raw DTO
// slices. It replaces the old Build(cfg config.Config) convenience that
// v2.1 dropped now that the gateway no longer owns a top-level Config
// struct.
func buildForTest(t *testing.T, providers []config.ProviderConfig, routes []config.RouteConfig) *Router {
	t.Helper()
	r := New()
	if err := r.Reload(providers, routes); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	return r
}

func TestBuildAndResolve(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk-a"},
		{Name: "deepseek", Kind: "deepseek", APIKey: "sk-b"},
		{Name: "claude", Kind: "anthropic", APIKey: "sk-c"},
	}
	routes := []config.RouteConfig{
		{Model: "gpt-4o", Provider: "openai", Fallback: []string{"deepseek"}},
		{Model: "claude-3-5-sonnet", Provider: "claude"},
	}
	r := buildForTest(t, providers, routes)

	chain, err := r.Resolve("gpt-4o")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(chain) != 2 || chain[0].Name() != "openai" || chain[1].Name() != "deepseek" {
		t.Errorf("unexpected chain: %v", chain)
	}

	if _, err := r.Resolve("unknown-model"); err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestBuild_OpenAICompatibleKinds(t *testing.T) {
	// Every entry in openaiCompatibleDefaults must round-trip through the
	// factory without error so operators discover typos at boot.
	kinds := make([]string, 0, len(openaiCompatibleDefaults))
	for k := range openaiCompatibleDefaults {
		kinds = append(kinds, k)
	}
	for _, kind := range kinds {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			providers := []config.ProviderConfig{
				{Name: kind, Kind: kind, APIKey: "sk-test"},
			}
			routes := []config.RouteConfig{{Model: "*", Provider: kind}}
			if err := New().Reload(providers, routes); err != nil {
				t.Fatalf("Reload(%s): %v", kind, err)
			}
		})
	}
}

func TestBuild_CatchAllRoute(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk-a"},
	}
	routes := []config.RouteConfig{
		{Model: "*", Provider: "openai"},
	}
	r := buildForTest(t, providers, routes)
	chain, err := r.Resolve("gpt-4o-mini")
	if err != nil {
		t.Fatalf("Resolve via catch-all: %v", err)
	}
	if len(chain) != 1 || chain[0].Name() != "openai" {
		t.Errorf("unexpected chain: %v", chain)
	}
}
