package bedrock

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider"
)

func TestAdapter_Chat_SignsRequestAndTranslates(t *testing.T) {
	var gotAuth string
	var gotBody []byte
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":{"message":{"role":"assistant","content":[{"text":"hi"}]}},
			"usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5},
			"stopReason":"end_turn"
		}`))
	}))
	defer server.Close()

	a, err := New(Options{
		Name:      "bedrock",
		Region:    "us-east-1",
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "SECRET",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := a.Chat(context.Background(), &provider.ChatRequest{
		Model: "anthropic.claude-3-5-sonnet-20241022-v2:0",
		Raw:   json.RawMessage(`{"model":"x","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hi"}],"max_tokens":64}`),
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(gotAuth, "AWS4-HMAC-SHA256") {
		t.Errorf("missing SigV4 auth header: %q", gotAuth)
	}
	if !strings.Contains(gotAuth, "Credential=AKIDEXAMPLE") {
		t.Errorf("auth missing credential: %q", gotAuth)
	}
	if !strings.Contains(gotPath, "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/converse") {
		t.Errorf("path: %s", gotPath)
	}
	if !strings.Contains(string(gotBody), `"system":[{"text":"be brief"}]`) {
		t.Errorf("system not lifted: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"maxTokens":64`) {
		t.Errorf("maxTokens missing: %s", gotBody)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Errorf("total tokens = %d", resp.Usage.TotalTokens)
	}
}

// encodeFrame builds a single EventStream binary frame for tests.
func encodeFrame(t *testing.T, messageType, eventType string, payload []byte) []byte {
	t.Helper()
	// Headers: ":message-type" (string), ":event-type" (string).
	var headers []byte
	headers = append(headers, encodeHeader(":message-type", messageType)...)
	if eventType != "" {
		headers = append(headers, encodeHeader(":event-type", eventType)...)
	}
	headerLen := uint32(len(headers))
	total := uint32(12 + len(headers) + len(payload) + 4)
	frame := make([]byte, 0, total)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, total)
	frame = append(frame, lenBuf...)
	binary.BigEndian.PutUint32(lenBuf, headerLen)
	frame = append(frame, lenBuf...)
	frame = append(frame, 0, 0, 0, 0) // prelude crc (ignored by parser)
	frame = append(frame, headers...)
	frame = append(frame, payload...)
	frame = append(frame, 0, 0, 0, 0) // message crc (ignored)
	return frame
}

func encodeHeader(name, value string) []byte {
	out := []byte{byte(len(name))}
	out = append(out, name...)
	out = append(out, 7) // string type
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(value)))
	out = append(out, lenBuf...)
	out = append(out, value...)
	return out
}

func TestAdapter_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		frames := [][]byte{
			encodeFrame(t, "event", "messageStart", []byte(`{"role":"assistant"}`)),
			encodeFrame(t, "event", "contentBlockDelta", []byte(`{"delta":{"text":"Hello"}}`)),
			encodeFrame(t, "event", "contentBlockDelta", []byte(`{"delta":{"text":" world"}}`)),
			encodeFrame(t, "event", "messageStop", []byte(`{"stopReason":"end_turn"}`)),
		}
		for _, f := range frames {
			_, _ = w.Write(f)
		}
	}))
	defer server.Close()

	a, err := New(Options{
		Name: "bedrock", Region: "us-east-1",
		AccessKey: "K", SecretKey: "S", BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := a.ChatStream(context.Background(), &provider.ChatRequest{
		Model: "anthropic.claude-3-5-sonnet-20241022-v2:0",
		Raw:   json.RawMessage(`{"model":"x","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var content strings.Builder
	var sawDone, sawFinish bool
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
				Delta        struct{ Content string `json:"content"` } `json:"delta"`
				FinishReason string                                    `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(ev.Data, &chunk); err != nil {
			t.Fatalf("decode chunk: %v", err)
		}
		for _, c := range chunk.Choices {
			content.WriteString(c.Delta.Content)
			if c.FinishReason != "" {
				sawFinish = true
			}
		}
	}
	if content.String() != "Hello world" {
		t.Errorf("content = %q", content.String())
	}
	if !sawDone {
		t.Error("missing [DONE]")
	}
	if !sawFinish {
		t.Error("missing finish_reason")
	}
}
