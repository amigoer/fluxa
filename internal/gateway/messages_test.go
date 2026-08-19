package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func toCanonical(t *testing.T, anthropicJSON string) chatRequest {
	t.Helper()
	inbound, err := decodeMessagesRequest(strings.NewReader(anthropicJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	req, err := inbound.toChatRequest()
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	return req
}

// Anthropic's max_tokens is required; a request without one would be
// refused upstream too, so it is refused here with a reason.
func TestMessagesRequiresMaxTokens(t *testing.T) {
	_, err := decodeMessagesRequest(strings.NewReader(`{"model":"m","messages":[]}`))
	if err == nil {
		t.Fatal("a request with no max_tokens was accepted")
	}
	if !strings.Contains(err.Error(), "max_tokens") {
		t.Errorf("err = %v", err)
	}
}

// The system prompt lives beside the conversation in Anthropic's shape
// and inside it in OpenAI's.
func TestMessagesLowersSystemIntoAMessage(t *testing.T) {
	req := toCanonical(t, `{"model":"m","max_tokens":100,"system":"be terse",
		"messages":[{"role":"user","content":"hi"}]}`)

	if len(req.Messages) != 2 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "be terse" {
		t.Errorf("first message = %+v", req.Messages[0])
	}

	// Anthropic also allows system as a list of text blocks.
	blocks := toCanonical(t, `{"model":"m","max_tokens":100,
		"system":[{"type":"text","text":"one"},{"type":"text","text":"two"}],
		"messages":[{"role":"user","content":"hi"}]}`)
	if blocks.Messages[0].Content != "one\n\ntwo" {
		t.Errorf("system from blocks = %q", blocks.Messages[0].Content)
	}
}

// The round trip that matters: a tool-calling conversation arrives in
// Anthropic's block form and has to come out in OpenAI's parallel-array
// form, including one tool message per result.
func TestMessagesTranslatesAToolCallingConversation(t *testing.T) {
	req := toCanonical(t, `{
		"model":"m","max_tokens":1024,
		"tools":[{"name":"get_weather","description":"look it up",
		          "input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
		"tool_choice":{"type":"auto"},
		"messages":[
			{"role":"user","content":"weather?"},
			{"role":"assistant","content":[
				{"type":"text","text":"checking"},
				{"type":"tool_use","id":"tu_1","name":"get_weather","input":{"city":"SF"}}]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"tu_1","content":"18C"}]}
		]}`)

	if len(req.Messages) != 3 {
		t.Fatalf("messages = %d: %+v", len(req.Messages), req.Messages)
	}

	assistant := req.Messages[1]
	if assistant.Role != "assistant" || assistant.Content != "checking" {
		t.Errorf("assistant = %+v", assistant)
	}
	var calls []openAIToolCall
	if err := json.Unmarshal(assistant.raw["tool_calls"], &calls); err != nil {
		t.Fatalf("tool_calls: %v", err)
	}
	if len(calls) != 1 || calls[0].ID != "tu_1" || calls[0].Function.Name != "get_weather" {
		t.Errorf("tool_calls = %+v", calls)
	}
	if calls[0].Function.Arguments != `{"city":"SF"}` {
		t.Errorf("arguments = %q", calls[0].Function.Arguments)
	}

	// A tool result becomes its own "tool" turn, naming the call it answers.
	result := req.Messages[2]
	if result.Role != "tool" || result.Content != "18C" {
		t.Errorf("tool turn = %+v", result)
	}
	if id, _ := jsonString(result.raw["tool_call_id"]); id != "tu_1" {
		t.Errorf("tool_call_id = %q", id)
	}

	var tools []struct {
		Function struct {
			Name       string          `json:"name"`
			Parameters json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(req.raw["tools"], &tools); err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Function.Name != "get_weather" {
		t.Errorf("tools = %+v", tools)
	}
	if !strings.Contains(string(tools[0].Function.Parameters), `"city"`) {
		t.Errorf("input_schema did not become parameters: %s", tools[0].Function.Parameters)
	}
	if string(req.raw["tool_choice"]) != `"auto"` {
		t.Errorf("tool_choice = %s", req.raw["tool_choice"])
	}
}

// Anthropic packs several tool results into one user turn; OpenAI wants
// one turn each.
func TestMessagesSplitsMultipleToolResults(t *testing.T) {
	req := toCanonical(t, `{"model":"m","max_tokens":10,"messages":[
		{"role":"user","content":[
			{"type":"tool_result","tool_use_id":"a","content":"1"},
			{"type":"tool_result","tool_use_id":"b","content":"2"}]}]}`)

	if len(req.Messages) != 2 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	for i, want := range []string{"a", "b"} {
		if req.Messages[i].Role != "tool" {
			t.Errorf("message %d role = %q", i, req.Messages[i].Role)
		}
		if id, _ := jsonString(req.Messages[i].raw["tool_call_id"]); id != want {
			t.Errorf("message %d tool_call_id = %q, want %q", i, id, want)
		}
	}
}

// An assistant turn that is only tool calls sends a null content in
// OpenAI's shape, not an empty string.
func TestMessagesKeepsAToolOnlyAssistantTurnContentless(t *testing.T) {
	req := toCanonical(t, `{"model":"m","max_tokens":10,"messages":[
		{"role":"assistant","content":[
			{"type":"tool_use","id":"tu","name":"f","input":{}}]}]}`)

	if len(req.Messages) != 1 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if _, present := req.Messages[0].raw["content"]; present {
		t.Errorf("content was set on a tool-only assistant turn: %s", req.Messages[0].raw["content"])
	}
}

func TestMessagesMapsToolChoice(t *testing.T) {
	base := `{"model":"m","max_tokens":10,"messages":[],"tool_choice":`
	for in, want := range map[string]string{
		`{"type":"auto"}`:                  `"auto"`,
		`{"type":"any"}`:                   `"required"`,
		`{"type":"none"}`:                  `"none"`,
		`{"type":"tool","name":"pick_me"}`: `{"type":"function","function":{"name":"pick_me"}}`,
	} {
		req := toCanonical(t, base+in+`}`)
		if got := string(req.raw["tool_choice"]); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

func TestMessagesCarriesSamplingParameters(t *testing.T) {
	req := toCanonical(t, `{"model":"m","max_tokens":64,"messages":[],
		"temperature":0.3,"top_p":0.9,"stop_sequences":["END"],"stream":true}`)

	if string(req.raw["temperature"]) != "0.3" || string(req.raw["top_p"]) != "0.9" {
		t.Errorf("sampling = %s / %s", req.raw["temperature"], req.raw["top_p"])
	}
	if string(req.raw["stop"]) != `["END"]` {
		t.Errorf("stop = %s", req.raw["stop"])
	}
	if !req.Stream || string(req.raw["stream"]) != "true" {
		t.Error("stream did not survive")
	}
	if req.MaxTokens != 64 {
		t.Errorf("max_tokens = %d", req.MaxTokens)
	}
}
