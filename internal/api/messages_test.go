package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/router"
)

func newAnthropicTestServer(t *testing.T, upstream *httptest.Server) *Server {
	t.Helper()
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080},
		Providers: []config.ProviderConfig{
			{Name: "claude", Kind: "anthropic", APIKey: "sk-ant", BaseURL: upstream.URL},
		},
		Routes: []config.RouteConfig{
			{Model: "claude-3-5-sonnet", Provider: "claude"},
		},
	}
	r, err := router.Build(cfg)
	if err != nil {
		t.Fatalf("router.Build: %v", err)
	}
	return New(r, nil, nil, nil)
}

func TestHandleMessages_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-3-5-sonnet","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":2,"output_tokens":5}}`))
	}))
	defer upstream.Close()

	s := newAnthropicTestServer(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/messages",
		strings.NewReader(`{"model":"claude-3-5-sonnet","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "claude-3-5-sonnet") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	if rec.Header().Get("X-Fluxa-Provider") != "claude" {
		t.Errorf("provider header = %q", rec.Header().Get("X-Fluxa-Provider"))
	}
}

func TestHandleMessages_RejectsNonAnthropicProvider(t *testing.T) {
	// OpenAI-only config → /v1/messages must return 501.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080},
		Providers: []config.ProviderConfig{
			{Name: "openai", Kind: "openai", APIKey: "sk", BaseURL: upstream.URL},
		},
		Routes: []config.RouteConfig{
			{Model: "gpt-4o", Provider: "openai"},
		},
	}
	r, err := router.Build(cfg)
	if err != nil {
		t.Fatalf("router.Build: %v", err)
	}
	s := New(r, nil, nil, nil)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/messages",
		strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}
