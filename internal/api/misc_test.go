package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/router"
)

func TestListModels(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	providers := []config.ProviderConfig{
		{Name: "openai", Kind: "openai", APIKey: "sk", BaseURL: upstream.URL},
	}
	routes := []config.RouteConfig{
		{Model: "gpt-4o", Provider: "openai"},
		{Model: "gpt-4o-mini", Provider: "openai"},
	}
	r := router.New()
	if err := r.Reload(providers, routes); err != nil {
		t.Fatalf("router.Reload: %v", err)
	}
	s := New(r, nil, nil, nil, nil)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Object != "list" || len(body.Data) != 2 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestHealth_AllOK(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	s := newTestServer(t, upstream)
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}
