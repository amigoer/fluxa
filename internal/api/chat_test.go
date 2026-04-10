package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/router"
)

func newTestServer(t *testing.T, upstream *httptest.Server) *Server {
	t.Helper()
	providers := []config.ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk-test", BaseURL: upstream.URL},
	}
	routes := []config.RouteConfig{
		{Model: "gpt-4o", Provider: "openai"},
	}
	r := router.New()
	if err := r.Reload(providers, routes); err != nil {
		t.Fatalf("router.Reload: %v", err)
	}
	return New(r, nil, nil, nil, nil)
}

func TestHandleChatCompletions_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-4o","choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"total_tokens":3}}`))
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "pong") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	if rec.Header().Get("X-Fluxa-Provider") != "openai" {
		t.Errorf("missing provider header: %q", rec.Header().Get("X-Fluxa-Provider"))
	}
}

func TestHandleChatCompletions_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, c := range []string{
			`{"choices":[{"delta":{"content":"hi"}}]}`,
			`{"choices":[{"delta":{"content":" there"}}]}`,
			`[DONE]`,
		} {
			_, _ = w.Write([]byte("data: " + c + "\n\n"))
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `data: {"choices":[{"delta":{"content":"hi"}}]}`) {
		t.Errorf("stream body missing first chunk: %s", body)
	}
	if !strings.Contains(string(body), "data: [DONE]") {
		t.Errorf("stream body missing DONE sentinel: %s", body)
	}
}

func TestHandleChatCompletions_BadRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	s := newTestServer(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
