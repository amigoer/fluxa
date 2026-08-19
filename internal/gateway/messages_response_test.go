package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// anthropicEvents parses the Anthropic SSE the writer produced back into
// (event name, payload) pairs.
func anthropicEvents(t *testing.T, stream string) [][2]string {
	t.Helper()
	var out [][2]string
	event := ""
	for _, line := range strings.Split(stream, "\n") {
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			out = append(out, [2]string{event, strings.TrimPrefix(line, "data: ")})
		}
	}
	return out
}

func eventNames(events [][2]string) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e[0]
	}
	return names
}

func TestMessagesResponseBecomesAnthropicShape(t *testing.T) {
	rec := httptest.NewRecorder()
	out := newMessagesWriter(rec, false, "claude-sonnet-4")

	out.WriteHeader(200)
	_, _ = out.Write([]byte(`{
		"id":"chatcmpl-1","model":"gpt-4o","choices":[{
			"finish_reason":"tool_calls",
			"message":{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]}}],
		"usage":{"prompt_tokens":30,"completion_tokens":11}}`))
	if err := out.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	var got struct {
		Type       string             `json:"type"`
		Role       string             `json:"role"`
		StopReason string             `json:"stop_reason"`
		Content    []anthropicContent `json:"content"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not Anthropic-shaped: %v\n%s", err, rec.Body.String())
	}

	if got.Type != "message" || got.Role != "assistant" {
		t.Errorf("envelope = %+v", got)
	}
	if got.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want tool_use", got.StopReason)
	}
	if len(got.Content) != 1 || got.Content[0].Type != "tool_use" {
		t.Fatalf("content = %+v", got.Content)
	}
	if got.Content[0].ID != "call_1" || got.Content[0].Name != "get_weather" {
		t.Errorf("tool_use = %+v", got.Content[0])
	}
	if string(got.Content[0].Input) != `{"city":"SF"}` {
		t.Errorf("input = %s", got.Content[0].Input)
	}
	if got.Usage.InputTokens != 30 || got.Usage.OutputTokens != 11 {
		t.Errorf("usage = %+v", got.Usage)
	}
}

func TestMessagesStreamProducesAnthropicEventSequence(t *testing.T) {
	rec := httptest.NewRecorder()
	out := newMessagesWriter(rec, true, "claude-sonnet-4")
	out.WriteHeader(200)

	chunks := []string{
		`{"id":"c1","model":"gpt-4o","choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":{"prompt_tokens":12,"completion_tokens":0}}`,
		`{"id":"c1","choices":[{"delta":{"content":"Hel"},"finish_reason":null}]}`,
		`{"id":"c1","choices":[{"delta":{"content":"lo"},"finish_reason":null}]}`,
		`{"id":"c1","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":5}}`,
	}
	for _, c := range chunks {
		_, _ = out.Write([]byte("data: " + c + "\n\n"))
	}
	_, _ = out.Write([]byte("data: [DONE]\n\n"))
	if err := out.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	events := anthropicEvents(t, rec.Body.String())
	names := eventNames(events)
	for _, want := range []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no %s event; got %v", want, names)
		}
	}
	if names[0] != "message_start" || names[len(names)-1] != "message_stop" {
		t.Errorf("sequence = %v", names)
	}

	var text strings.Builder
	for _, e := range events {
		if e[0] != "content_block_delta" {
			continue
		}
		var ev struct {
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(e[1]), &ev); err != nil {
			t.Fatalf("delta: %v", err)
		}
		text.WriteString(ev.Delta.Text)
	}
	if text.String() != "Hello" {
		t.Errorf("reassembled text = %q", text.String())
	}

	// message_delta carries the stop reason and the output token count.
	for _, e := range events {
		if e[0] != "message_delta" {
			continue
		}
		var ev struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(e[1]), &ev); err != nil {
			t.Fatalf("message_delta: %v", err)
		}
		if ev.Delta.StopReason != "end_turn" {
			t.Errorf("stop_reason = %q", ev.Delta.StopReason)
		}
		if ev.Usage.OutputTokens != 5 {
			t.Errorf("output_tokens = %d", ev.Usage.OutputTokens)
		}
	}
}

// Tool calls become their own content blocks, and text keeps block 0
// whatever order the OpenAI stream interleaved them in.
func TestMessagesStreamRebuildsToolUseBlocks(t *testing.T) {
	rec := httptest.NewRecorder()
	out := newMessagesWriter(rec, true, "m")
	out.WriteHeader(200)

	for _, c := range []string{
		`{"id":"c","choices":[{"delta":{"content":"checking"}}]}`,
		`{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"id":"tu_1","type":"function","function":{"name":"f","arguments":""}}]}}]}`,
		`{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"a\":"}}]}}]}`,
		`{"id":"c","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
		`{"id":"c","choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	} {
		_, _ = out.Write([]byte("data: " + c + "\n\n"))
	}
	if err := out.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	events := anthropicEvents(t, rec.Body.String())

	var toolBlockIndex = -1
	var args strings.Builder
	for _, e := range events {
		switch e[0] {
		case "content_block_start":
			var ev struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(e[1]), &ev); err != nil {
				t.Fatalf("block start: %v", err)
			}
			if ev.ContentBlock.Type == "tool_use" {
				toolBlockIndex = ev.Index
				if ev.ContentBlock.ID != "tu_1" || ev.ContentBlock.Name != "f" {
					t.Errorf("tool_use block = %+v", ev.ContentBlock)
				}
			}
		case "content_block_delta":
			var ev struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(e[1]), &ev); err != nil {
				t.Fatalf("block delta: %v", err)
			}
			if ev.Delta.Type == "input_json_delta" {
				args.WriteString(ev.Delta.PartialJSON)
			}
		}
	}

	if toolBlockIndex <= 0 {
		t.Errorf("tool block index = %d, want text to keep block 0", toolBlockIndex)
	}
	if args.String() != `{"a":1}` {
		t.Errorf("reassembled arguments = %q", args.String())
	}
}

// Writes are not guaranteed to arrive frame-aligned -- an adapter writes
// the prefix, the payload and the terminator separately, and a network
// read can split anywhere. A translation that assumed whole frames per
// Write would corrupt the stream under exactly the conditions that only
// show up in production.
func TestMessagesStreamHandlesWritesSplitMidFrame(t *testing.T) {
	full := `data: {"id":"c","choices":[{"delta":{"content":"Hello there"}}]}` + "\n\n" +
		`data: {"id":"c","choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n"

	for _, chunkSize := range []int{1, 3, 7, 17, 64} {
		rec := httptest.NewRecorder()
		out := newMessagesWriter(rec, true, "m")
		out.WriteHeader(200)

		for i := 0; i < len(full); i += chunkSize {
			end := min(i+chunkSize, len(full))
			if _, err := out.Write([]byte(full[i:end])); err != nil {
				t.Fatalf("chunk size %d: %v", chunkSize, err)
			}
		}
		if err := out.Finish(); err != nil {
			t.Fatalf("chunk size %d finish: %v", chunkSize, err)
		}

		var text strings.Builder
		for _, e := range anthropicEvents(t, rec.Body.String()) {
			if e[0] != "content_block_delta" {
				continue
			}
			var ev struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			_ = json.Unmarshal([]byte(e[1]), &ev)
			text.WriteString(ev.Delta.Text)
		}
		if text.String() != "Hello there" {
			t.Errorf("chunk size %d: reassembled %q, want %q", chunkSize, text.String(), "Hello there")
		}
	}
}

// An upstream error is the one thing worth passing through in whatever
// words the provider used.
func TestMessagesRelaysAnUpstreamErrorUntranslated(t *testing.T) {
	rec := httptest.NewRecorder()
	out := newMessagesWriter(rec, false, "m")

	out.WriteHeader(429)
	_, _ = out.Write([]byte(`{"error":{"message":"slow down"}}`))
	if err := out.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	if rec.Code != 429 {
		t.Errorf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "slow down") {
		t.Errorf("body = %s", rec.Body.String())
	}
}
