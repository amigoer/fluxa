package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// decodeBody runs a caller's JSON through the same path the handler does
// and returns what would actually be sent upstream, parsed back out.
func decodeBody(t *testing.T, clientJSON, modelIdentifier string) map[string]json.RawMessage {
	t.Helper()
	req, err := decodeChatRequest(strings.NewReader(clientJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := req.body(bodyOptions{ModelIdentifier: modelIdentifier})
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse upstream body: %v", err)
	}
	return out
}

// Forwarding used to re-marshal a four-field struct, which dropped every
// parameter that struct didn't list. tools/tool_choice going missing is
// the worst of it: function calling stopped working with no error.
func TestForwardKeepsParametersTheGatewayDoesNotModel(t *testing.T) {
	client := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"temperature": 0.2,
		"top_p": 0.9,
		"stop": ["\n\n"],
		"seed": 42,
		"presence_penalty": 0.5,
		"n": 2,
		"user": "emp-123",
		"response_format": {"type": "json_object"},
		"stream_options": {"include_usage": true},
		"tool_choice": "auto",
		"tools": [{"type": "function", "function": {"name": "get_weather"}}]
	}`

	got := decodeBody(t, client, "gpt-4o-2024-11-20")

	for field, want := range map[string]string{
		"temperature":      `0.2`,
		"top_p":            `0.9`,
		"stop":             `["\n\n"]`,
		"seed":             `42`,
		"presence_penalty": `0.5`,
		"n":                `2`,
		"user":             `"emp-123"`,
		"response_format":  `{"type": "json_object"}`,
		"stream_options":   `{"include_usage": true}`,
		"tool_choice":      `"auto"`,
		"tools":            `[{"type": "function", "function": {"name": "get_weather"}}]`,
	} {
		v, ok := got[field]
		if !ok {
			t.Errorf("%s was dropped on the way upstream", field)
			continue
		}
		// Values are relayed unchanged but re-encoded, so insignificant
		// whitespace inside them is not preserved; compare compacted.
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, []byte(want)); err != nil {
			t.Fatalf("compact %s: %v", field, err)
		}
		if string(v) != compacted.String() {
			t.Errorf("%s = %s, want %s", field, v, compacted.String())
		}
	}
}

// An unset max_tokens used to be serialized as 0 because the struct
// field had no omitempty -- an upstream that requires max_tokens >= 1
// rejects that outright.
func TestForwardOmitsMaxTokensTheCallerNeverSet(t *testing.T) {
	got := decodeBody(t, `{"model":"m","messages":[{"role":"user","content":"hi"}]}`, "m-1")
	if v, ok := got["max_tokens"]; ok {
		t.Errorf("max_tokens = %s, want the field absent entirely", v)
	}
}

func TestForwardKeepsMaxTokensTheCallerDidSet(t *testing.T) {
	got := decodeBody(t, `{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":256}`, "m-1")
	if string(got["max_tokens"]) != "256" {
		t.Errorf("max_tokens = %s, want 256", got["max_tokens"])
	}
}

// The caller names a model as Fluxa knows it; the provider is told its
// own identifier for whichever model routing actually picked.
func TestForwardSwapsInTheProvidersModelIdentifier(t *testing.T) {
	got := decodeBody(t, `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`, "gpt-4o-mini")
	if string(got["model"]) != `"gpt-4o-mini"` {
		t.Errorf("model = %s, want \"gpt-4o-mini\"", got["model"])
	}
}

// Masking rewrites content and nothing else: an assistant turn's
// tool_calls and a message's name have to survive the round trip, or
// masking one message breaks the conversation.
func TestMaskingAMessageKeepsItsOtherFields(t *testing.T) {
	client := `{
		"model": "m",
		"messages": [
			{"role": "user", "content": "id 110101199003078515", "name": "alice"},
			{"role": "assistant", "content": null,
			 "tool_calls": [{"id": "call_1", "type": "function",
			                 "function": {"name": "lookup", "arguments": "{}"}}]},
			{"role": "tool", "content": "ok", "tool_call_id": "call_1"}
		]
	}`

	req, err := decodeChatRequest(strings.NewReader(client))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	req.Messages[0].Content = "id 1****************5" // what Scan would hand back

	raw, err := req.body(bodyOptions{ModelIdentifier: "m-1"})
	if err != nil {
		t.Fatalf("body: %v", err)
	}

	var out struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(out.Messages))
	}

	if string(out.Messages[0]["content"]) != `"id 1****************5"` {
		t.Errorf("masked content = %s", out.Messages[0]["content"])
	}
	if string(out.Messages[0]["name"]) != `"alice"` {
		t.Errorf("name = %s, want \"alice\"", out.Messages[0]["name"])
	}
	if _, ok := out.Messages[1]["tool_calls"]; !ok {
		t.Error("tool_calls was dropped from the assistant message")
	}
	// An assistant turn that carries only tool_calls sends content null;
	// rewriting that to "" tells the provider something different.
	if string(out.Messages[1]["content"]) != "null" {
		t.Errorf("content = %s, want null to be left alone", out.Messages[1]["content"])
	}
	if string(out.Messages[2]["tool_call_id"]) != `"call_1"` {
		t.Errorf("tool_call_id = %s", out.Messages[2]["tool_call_id"])
	}
}

// Multimodal content parts are refused rather than relayed: passing an
// array through untouched would mean DLP never reads the text in it.
func TestContentPartsAreRefusedNotSilentlyUnscanned(t *testing.T) {
	client := `{"model":"m","messages":[{"role":"user","content":[
		{"type":"text","text":"id 110101199003078515"},
		{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}
	]}]}`

	if _, err := decodeChatRequest(strings.NewReader(client)); !errors.Is(err, errContentParts) {
		t.Fatalf("err = %v, want errContentParts", err)
	}
}

func TestEstimateMessageTokensCountsEveryMessage(t *testing.T) {
	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[{"role":"system","content":"aaaa"},{"role":"user","content":"bbbbbbbb"}]}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 4 runes -> 2, 8 runes -> 3
	if got := estimateMessageTokens(req.Messages); got != 5 {
		t.Errorf("estimateMessageTokens = %d, want 5", got)
	}
}

// The gateway needs usage on streamed calls to bill from, so it asks for
// it whether or not the caller did.
func TestForwardAsksTheProviderForStreamingUsage(t *testing.T) {
	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := req.body(bodyOptions{ModelIdentifier: "m-1", RequestUsage: true})
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if string(out["stream_options"]) != `{"include_usage":true}` {
		t.Errorf("stream_options = %s, want include_usage on", out["stream_options"])
	}
	if req.wantsUsageFrame() {
		t.Error("wantsUsageFrame reported true for a caller that set no stream_options")
	}
}

// Turning include_usage on must not discard the rest of what the caller
// put in stream_options.
func TestAskingForUsageKeepsTheCallersOtherStreamOptions(t *testing.T) {
	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[],"stream":true,"stream_options":{"include_usage":false,"something_else":7}}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.wantsUsageFrame() {
		t.Error("wantsUsageFrame reported true for include_usage: false")
	}

	raw, err := req.body(bodyOptions{ModelIdentifier: "m-1", RequestUsage: true})
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	var out struct {
		StreamOptions map[string]json.RawMessage `json:"stream_options"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if string(out.StreamOptions["include_usage"]) != "true" {
		t.Errorf("include_usage = %s, want true", out.StreamOptions["include_usage"])
	}
	if string(out.StreamOptions["something_else"]) != "7" {
		t.Errorf("something_else = %s, want it preserved", out.StreamOptions["something_else"])
	}
}

func TestCallerThatAskedForUsageIsRecognised(t *testing.T) {
	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[],"stream":true,"stream_options":{"include_usage":true}}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !req.wantsUsageFrame() {
		t.Error("wantsUsageFrame reported false for include_usage: true")
	}
}

// Dividing every character by four is an English-only rule; a Chinese
// prompt estimated that way came out several times under, and that
// number gates spending.
func TestEstimateTokensDoesNotUndercountChinese(t *testing.T) {
	// 40 Han characters.
	chinese := strings.Repeat("这是一段中文提示词内容需要被正确估算", 2) + "这是一段"
	runes := len([]rune(chinese))

	got := estimateTokens(chinese)
	if old := runes/4 + 1; got <= old {
		t.Errorf("estimate = %d, still at or below the old character/4 figure of %d", got, old)
	}
	if got < runes {
		t.Errorf("estimate = %d for %d Han characters, want at least one token each", got, runes)
	}
}

func TestEstimateTokensStillTreatsLatinTextAsBefore(t *testing.T) {
	english := strings.Repeat("the quick brown fox ", 10) // 200 chars
	if got, want := estimateTokens(english), 200/4+1; got != want {
		t.Errorf("estimate = %d, want %d", got, want)
	}
}

func TestEstimateTokensHandlesMixedText(t *testing.T) {
	// 4 Han characters + 8 Latin characters -> 4 + 2 + 1
	if got := estimateTokens("中文混排 abcdefgh"); got != 4+9/4+1 {
		t.Errorf("estimate = %d, want %d", got, 4+9/4+1)
	}
}
