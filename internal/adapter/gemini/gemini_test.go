package gemini

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

func TestAdapter_Chat_Translation(t *testing.T) {
	var gotPath string
	var gotBody []byte
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-goog-api-key")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates":[{"content":{"parts":[{"text":"Hello from Gemini"}]},"finishReason":"STOP"}],
			"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":8,"totalTokenCount":12}
		}`))
	}))
	defer server.Close()

	a := New(Options{Name: "gemini", BaseURL: server.URL, APIKey: "goog-key"})
	resp, err := a.Chat(context.Background(), &provider.ChatRequest{
		Model: "gemini-2.0-flash",
		Raw: json.RawMessage(`{
			"model":"gemini-2.0-flash",
			"messages":[
				{"role":"system","content":"Be brief"},
				{"role":"user","content":"hi"}
			],
			"temperature":0.7,
			"max_tokens":200
		}`),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(gotPath, "/models/gemini-2.0-flash:generateContent") {
		t.Errorf("path missing model:action: %s", gotPath)
	}
	if gotAPIKey != "goog-key" {
		t.Errorf("missing api key header")
	}
	if !strings.Contains(string(gotBody), `"systemInstruction"`) {
		t.Errorf("system instruction not lifted: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"maxOutputTokens":200`) {
		t.Errorf("max_tokens not translated: %s", gotBody)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Raw, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Choices) != 1 || payload.Choices[0].Message.Content != "Hello from Gemini" {
		t.Errorf("unexpected content: %+v", payload.Choices)
	}
	if payload.Choices[0].FinishReason != "stop" {
		t.Errorf("finish reason = %q", payload.Choices[0].FinishReason)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Errorf("total_tokens = %d", resp.Usage.TotalTokens)
	}
}

func TestAdapter_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, payload := range []string{
			`{"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`,
			`{"candidates":[{"content":{"parts":[{"text":" there"}]},"finishReason":"STOP"}]}`,
		} {
			_, _ = w.Write([]byte("data: " + payload + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	a := New(Options{Name: "gemini", BaseURL: server.URL, APIKey: "k"})
	ch, err := a.ChatStream(context.Background(), &provider.ChatRequest{
		Model: "gemini-2.0-flash",
		Raw:   json.RawMessage(`{"model":"gemini-2.0-flash","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var content strings.Builder
	var sawDone bool
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream err: %v", ev.Err)
		}
		if string(ev.Data) == "[DONE]" {
			sawDone = true
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(ev.Data, &chunk); err != nil {
			t.Fatalf("decode chunk: %v (%s)", err, ev.Data)
		}
		for _, c := range chunk.Choices {
			content.WriteString(c.Delta.Content)
		}
	}
	if content.String() != "Hi there" {
		t.Errorf("stream content = %q", content.String())
	}
	if !sawDone {
		t.Error("missing [DONE] sentinel")
	}
}
