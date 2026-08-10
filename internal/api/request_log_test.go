package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/router"
	"github.com/amigoer/fluxa/internal/store"
	"github.com/amigoer/fluxa/internal/testdb"
)

// newTestServerWithStore boots a Server with a real on-disk store so
// request_log inserts can be inspected after the handler runs.
func newTestServerWithStore(t *testing.T, upstream *httptest.Server) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(t.Context(), testdb.New(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	providers := []config.ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk-test", BaseURL: upstream.URL},
	}
	routes := []config.RouteConfig{{Model: "gpt-4o", Provider: "openai"}}
	r := router.New()
	if err := r.Reload(providers, routes); err != nil {
		t.Fatalf("router.Reload: %v", err)
	}
	return New(r, nil, nil, st, nil), st
}

func TestRequestLog_NonStreamSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`))
	}))
	defer upstream.Close()

	s, st := newTestServerWithStore(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fluxa-test/1.0")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	logs, total, err := st.ListRequestLogs(req.Context(), store.RequestLogFilter{})
	if err != nil {
		t.Fatalf("ListRequestLogs: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expected 1 row, got len=%d total=%d", len(logs), total)
	}
	got := logs[0]
	if got.Endpoint != "/v1/chat/completions" {
		t.Errorf("endpoint mismatch: %s", got.Endpoint)
	}
	if got.ModelRequested != "gpt-4o" || got.ModelResolved != "gpt-4o" {
		t.Errorf("model mismatch: req=%s res=%s", got.ModelRequested, got.ModelResolved)
	}
	if got.Provider != "openai" {
		t.Errorf("provider mismatch: %s", got.Provider)
	}
	if got.IsStream {
		t.Errorf("expected non-stream")
	}
	if got.StatusCode != 200 {
		t.Errorf("status_code = %d", got.StatusCode)
	}
	if got.PromptTokens != 3 || got.CompletionTokens != 1 || got.TotalTokens != 4 {
		t.Errorf("tokens mismatch: %+v", got)
	}
	if !strings.Contains(got.RequestBody, "ping") {
		t.Errorf("request body not captured: %q", got.RequestBody)
	}
	if !strings.Contains(got.ResponseBody, "pong") {
		t.Errorf("response body not captured: %q", got.ResponseBody)
	}
	if got.UserAgent != "fluxa-test/1.0" {
		t.Errorf("user_agent mismatch: %s", got.UserAgent)
	}
	if got.Error != "" {
		t.Errorf("expected empty error, got %q", got.Error)
	}
}

func TestRequestLog_StreamSuccess(t *testing.T) {
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

	s, st := newTestServerWithStore(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	logs, _, err := st.ListRequestLogs(req.Context(), store.RequestLogFilter{})
	if err != nil {
		t.Fatalf("ListRequestLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 row, got %d", len(logs))
	}
	got := logs[0]
	if !got.IsStream {
		t.Errorf("expected is_stream true")
	}
	if got.Provider != "openai" {
		t.Errorf("provider mismatch: %s", got.Provider)
	}
	if got.FirstByteAt == nil {
		t.Errorf("first_byte_at should be populated for streaming responses")
	}
	if !strings.Contains(got.ResponseBody, "delta") {
		t.Errorf("streamed response body not captured: %q", got.ResponseBody)
	}
}

func TestRequestLog_UnknownModelLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()

	s, st := newTestServerWithStore(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"no-such-model","messages":[]}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	logs, _, err := st.ListRequestLogs(req.Context(), store.RequestLogFilter{})
	if err != nil {
		t.Fatalf("ListRequestLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 row, got %d", len(logs))
	}
	got := logs[0]
	if got.StatusCode != http.StatusNotFound {
		t.Errorf("status_code = %d", got.StatusCode)
	}
	if got.Error == "" {
		t.Errorf("expected error message captured")
	}
}
