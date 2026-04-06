package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider"
)

func TestAdapter_Chat_UsesDeploymentAndApiKeyHeader(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotAPIKey = r.Header.Get("api-key")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-4o","usage":{"total_tokens":3}}`))
	}))
	defer server.Close()

	a, err := New(Options{
		Name:        "azure",
		BaseURL:     server.URL,
		APIKey:      "sk-azure",
		APIVersion:  "2024-06-01",
		Deployments: map[string]string{"gpt-4o": "prod-gpt4o"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Chat(context.Background(), &provider.ChatRequest{
		Model: "gpt-4o",
		Raw:   json.RawMessage(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(gotPath, "/openai/deployments/prod-gpt4o/chat/completions") {
		t.Errorf("path missing deployment: %s", gotPath)
	}
	if !strings.Contains(gotPath, "api-version=2024-06-01") {
		t.Errorf("path missing api-version: %s", gotPath)
	}
	if gotAPIKey != "sk-azure" {
		t.Errorf("api-key header = %q", gotAPIKey)
	}
	if strings.Contains(string(gotBody), `"model"`) {
		t.Errorf("body should not include model field for Azure: %s", gotBody)
	}
}

func TestAdapter_UnknownModel(t *testing.T) {
	a, err := New(Options{
		Name:        "azure",
		BaseURL:     "https://x.openai.azure.com",
		APIKey:      "sk",
		Deployments: map[string]string{"gpt-4o": "prod"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = a.Chat(context.Background(), &provider.ChatRequest{
		Model: "unknown",
		Raw:   json.RawMessage(`{"model":"unknown"}`),
	})
	var pErr *provider.Error
	if err == nil || !asProviderError(err, &pErr) || pErr.Status != http.StatusNotFound {
		t.Errorf("want 404 provider.Error, got: %v", err)
	}
}

func asProviderError(err error, target **provider.Error) bool {
	pe, ok := err.(*provider.Error)
	if !ok {
		return false
	}
	*target = pe
	return true
}
