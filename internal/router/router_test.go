package router

import (
	"testing"

	"github.com/amigoer/fluxa/internal/config"
)

func TestBuildAndResolve(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080},
		Providers: []config.ProviderConfig{
			{Name: "openai", Kind: "openai", APIKey: "sk-a"},
			{Name: "deepseek", Kind: "deepseek", APIKey: "sk-b"},
			{Name: "claude", Kind: "anthropic", APIKey: "sk-c"},
		},
		Routes: []config.RouteConfig{
			{Model: "gpt-4o", Provider: "openai", Fallback: []string{"deepseek"}},
			{Model: "claude-3-5-sonnet", Provider: "claude"},
		},
	}
	r, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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
	kinds := []string{"openai", "deepseek", "qwen", "ollama", "moonshot", "zhipu", "doubao", "ernie"}
	for _, kind := range kinds {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			cfg := config.Config{
				Server: config.ServerConfig{Port: 8080},
				Providers: []config.ProviderConfig{
					{Name: kind, Kind: kind, APIKey: "sk-test"},
				},
				Routes: []config.RouteConfig{{Model: "*", Provider: kind}},
			}
			if _, err := Build(cfg); err != nil {
				t.Fatalf("Build(%s): %v", kind, err)
			}
		})
	}
}

func TestBuild_CatchAllRoute(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080},
		Providers: []config.ProviderConfig{
			{Name: "openai", Kind: "openai", APIKey: "sk-a"},
		},
		Routes: []config.RouteConfig{
			{Model: "*", Provider: "openai"},
		},
	}
	r, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	chain, err := r.Resolve("gpt-4o-mini")
	if err != nil {
		t.Fatalf("Resolve via catch-all: %v", err)
	}
	if len(chain) != 1 || chain[0].Name() != "openai" {
		t.Errorf("unexpected chain: %v", chain)
	}
}
