package openai

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

func TestAdapter_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"gpt-4o"`) {
			t.Errorf("body missing model: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12},"choices":[]}`))
	}))
	defer server.Close()

	a := New(Options{Name: "openai", BaseURL: server.URL, APIKey: "sk-test"})
	raw := json.RawMessage(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	resp, err := a.Chat(context.Background(), &provider.ChatRequest{Model: "gpt-4o", Raw: raw})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Errorf("total_tokens = %d, want 12", resp.Usage.TotalTokens)
	}
}

func TestAdapter_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`{"choices":[{"delta":{"content":"hello"}}]}`,
			`{"choices":[{"delta":{"content":" world"}}]}`,
			`[DONE]`,
		}
		for _, c := range chunks {
			_, _ = w.Write([]byte("data: " + c + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	a := New(Options{Name: "openai", BaseURL: server.URL, APIKey: "sk-test"})
	ch, err := a.ChatStream(context.Background(), &provider.ChatRequest{
		Model: "gpt-4o",
		Raw:   json.RawMessage(`{"model":"gpt-4o","messages":[]}`),
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var got []string
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream err: %v", ev.Err)
		}
		got = append(got, string(ev.Data))
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(got), got)
	}
	if got[2] != "[DONE]" {
		t.Errorf("missing sentinel, got %q", got[2])
	}
}

func TestAdapter_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down","code":"rate_limited"}}`))
	}))
	defer server.Close()

	a := New(Options{Name: "openai", BaseURL: server.URL, APIKey: "sk"})
	_, err := a.Chat(context.Background(), &provider.ChatRequest{
		Raw: json.RawMessage(`{"model":"gpt-4o"}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var pErr *provider.Error
	if !asProviderError(err, &pErr) || pErr.Status != http.StatusTooManyRequests {
		t.Errorf("unexpected error: %T %v", err, err)
	}
}

// asProviderError is a tiny helper to avoid importing errors in test boiler.
func asProviderError(err error, target **provider.Error) bool {
	pe, ok := err.(*provider.Error)
	if !ok {
		return false
	}
	*target = pe
	return true
}
