package anthropic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider"
)

func TestAdapter_Messages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			t.Errorf("missing api key header: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("missing version header: %q", r.Header.Get("anthropic-version"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `claude-3-5-sonnet`) {
			t.Errorf("body missing model: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-3-5-sonnet","usage":{"input_tokens":10,"output_tokens":20},"content":[{"type":"text","text":"hi"}]}`))
	}))
	defer server.Close()

	a := New(Options{Name: "anthropic", BaseURL: server.URL, APIKey: "sk-ant-test"})
	resp, err := a.Messages(context.Background(), &provider.MessagesRequest{
		Model: "claude-3-5-sonnet",
		Raw:   []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("total_tokens = %d, want 30", resp.Usage.TotalTokens)
	}
}

func TestAdapter_MessagesStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		frames := []string{
			"event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			"event: content_block_delta\ndata: {\"delta\":{\"text\":\"hi\"}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}
		for _, f := range frames {
			_, _ = w.Write([]byte(f))
			flusher.Flush()
		}
	}))
	defer server.Close()

	a := New(Options{Name: "anthropic", BaseURL: server.URL, APIKey: "sk-ant-test"})
	ch, err := a.MessagesStream(context.Background(), &provider.MessagesRequest{
		Raw: []byte(`{"model":"claude-3-5-sonnet","messages":[]}`),
	})
	if err != nil {
		t.Fatalf("MessagesStream: %v", err)
	}
	var count int
	var sawStop bool
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream err: %v", ev.Err)
		}
		count++
		if strings.HasPrefix(string(ev.Data), "message_stop|") {
			sawStop = true
		}
	}
	if count != 3 {
		t.Errorf("expected 3 events, got %d", count)
	}
	if !sawStop {
		t.Error("missing message_stop event")
	}
}
