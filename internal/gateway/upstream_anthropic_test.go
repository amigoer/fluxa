package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

func anthropicProvider(t *testing.T, handler http.HandlerFunc) (types.Provider, *[]byte) {
	t.Helper()
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return types.Provider{
		Name:   "claude",
		Kind:   types.ProviderKindAnthropic,
		Config: map[string]any{"base_url": srv.URL, "api_key": "sk-ant-test"},
	}, &received
}

func translate(t *testing.T, clientJSON string) anthropicRequest {
	t.Helper()
	req, err := decodeChatRequest(strings.NewReader(clientJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := anthropicBody(req, types.Model{ModelIdentifier: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var out anthropicRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	return out
}

// Anthropic carries the system prompt beside the conversation, not as a
// message in it.
func TestAnthropicHoistsSystemMessages(t *testing.T) {
	got := translate(t, `{"model":"m","messages":[
		{"role":"system","content":"be terse"},
		{"role":"system","content":"answer in English"},
		{"role":"user","content":"hi"}]}`)

	if got.System != "be terse\n\nanswer in English" {
		t.Errorf("system = %q", got.System)
	}
	if len(got.Messages) != 1 || got.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want only the user turn", got.Messages)
	}
}

// Anthropic requires max_tokens; OpenAI treats it as optional.
func TestAnthropicAlwaysSendsMaxTokens(t *testing.T) {
	if got := translate(t, `{"model":"m","messages":[{"role":"user","content":"hi"}]}`); got.MaxTokens != anthropicDefaultMaxTokens {
		t.Errorf("max_tokens = %d, want the default %d", got.MaxTokens, anthropicDefaultMaxTokens)
	}
	if got := translate(t, `{"model":"m","messages":[{"role":"user","content":"hi"}],"max_tokens":256}`); got.MaxTokens != 256 {
		t.Errorf("max_tokens = %d, want the caller's 256", got.MaxTokens)
	}
}

// A whole tool-calling round trip: declare a tool, the assistant calls
// it, the result comes back. All three shapes differ between the APIs
// and dropping any of them breaks the conversation silently.
func TestAnthropicTranslatesAToolCallingConversation(t *testing.T) {
	got := translate(t, `{
		"model":"m",
		"messages":[
			{"role":"user","content":"weather in SF?"},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"18C"}
		],
		"tools":[{"type":"function","function":{
			"name":"get_weather","description":"look up weather",
			"parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
		"tool_choice":"auto"
	}`)

	if len(got.Tools) != 1 || got.Tools[0].Name != "get_weather" {
		t.Fatalf("tools = %+v", got.Tools)
	}
	if !strings.Contains(string(got.Tools[0].InputSchema), `"city"`) {
		t.Errorf("input_schema = %s, want the parameters carried over", got.Tools[0].InputSchema)
	}
	if string(got.ToolChoice) != `{"type":"auto"}` {
		t.Errorf("tool_choice = %s", got.ToolChoice)
	}

	if len(got.Messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(got.Messages))
	}
	assistant := got.Messages[1]
	if assistant.Role != "assistant" || len(assistant.Content) != 1 || assistant.Content[0].Type != "tool_use" {
		t.Fatalf("assistant turn = %+v", assistant)
	}
	if assistant.Content[0].ID != "call_1" || assistant.Content[0].Name != "get_weather" {
		t.Errorf("tool_use = %+v", assistant.Content[0])
	}
	if string(assistant.Content[0].Input) != `{"city":"SF"}` {
		t.Errorf("tool input = %s", assistant.Content[0].Input)
	}

	// A tool result is a user turn carrying a tool_result block, and it
	// has to name the call it answers.
	result := got.Messages[2]
	if result.Role != "user" || result.Content[0].Type != "tool_result" {
		t.Fatalf("tool result turn = %+v", result)
	}
	if result.Content[0].ToolUseID != "call_1" || result.Content[0].Content != "18C" {
		t.Errorf("tool_result = %+v", result.Content[0])
	}
}

func TestAnthropicTranslatesToolChoiceVariants(t *testing.T) {
	base := `{"model":"m","messages":[],"tools":[{"type":"function","function":{"name":"f","parameters":{}}}],`

	if got := translate(t, base+`"tool_choice":"required"}`); string(got.ToolChoice) != `{"type":"any"}` {
		t.Errorf("required -> %s, want {\"type\":\"any\"}", got.ToolChoice)
	}
	got := translate(t, base+`"tool_choice":{"type":"function","function":{"name":"f"}}}`)
	if string(got.ToolChoice) != `{"type":"tool","name":"f"}` {
		t.Errorf("named -> %s", got.ToolChoice)
	}
	// Anthropic has no "none"; withholding the tools says the same thing.
	if got := translate(t, base+`"tool_choice":"none"}`); len(got.Tools) != 0 {
		t.Errorf("none left %d tools declared, want them withheld", len(got.Tools))
	}
}

func TestAnthropicCarriesSamplingParameters(t *testing.T) {
	got := translate(t, `{"model":"m","messages":[],"temperature":0.3,"top_p":0.8,"stop":["\n\n","END"]}`)
	if got.Temperature == nil || *got.Temperature != 0.3 {
		t.Errorf("temperature = %v", got.Temperature)
	}
	if got.TopP == nil || *got.TopP != 0.8 {
		t.Errorf("top_p = %v", got.TopP)
	}
	if len(got.StopSequences) != 2 || got.StopSequences[1] != "END" {
		t.Errorf("stop_sequences = %v", got.StopSequences)
	}
	// OpenAI also allows a bare string.
	if one := translate(t, `{"model":"m","messages":[],"stop":"END"}`); len(one.StopSequences) != 1 {
		t.Errorf("stop as a string -> %v", one.StopSequences)
	}
}

func TestAnthropicNonStreamingResponseBecomesOpenAIShape(t *testing.T) {
	provider, _ := anthropicProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"id":"msg_1","model":"claude-sonnet-4-20250514","stop_reason":"tool_use",
			"content":[{"type":"text","text":"looking that up"},
			           {"type":"tool_use","id":"tu_1","name":"get_weather","input":{"city":"SF"}}],
			"usage":{"input_tokens":42,"output_tokens":17}}`)
	})

	req, err := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rec := httptest.NewRecorder()
	got, err := newUpstreamClient().forward(context.Background(), provider, types.Model{ModelIdentifier: "claude-sonnet-4"}, req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if got.Usage == nil || got.Usage.PromptTokens != 42 || got.Usage.CompletionTokens != 17 {
		t.Errorf("usage = %+v, want 42/17", got.Usage)
	}

	var out struct {
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string           `json:"role"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage chatUsage `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not OpenAI-shaped: %v\n%s", err, rec.Body.String())
	}
	if out.Object != "chat.completion" {
		t.Errorf("object = %q", out.Object)
	}
	if out.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want the name the provider reported", out.Model)
	}
	if len(out.Choices) != 1 || out.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("choices = %+v, want finish_reason tool_calls", out.Choices)
	}
	calls := out.Choices[0].Message.ToolCalls
	if len(calls) != 1 || calls[0].ID != "tu_1" || calls[0].Function.Name != "get_weather" {
		t.Errorf("tool_calls = %+v", calls)
	}
	if calls[0].Function.Arguments != `{"city":"SF"}` {
		t.Errorf("arguments = %q", calls[0].Function.Arguments)
	}
}

func TestAnthropicRelaysItsOwnErrorsUnchanged(t *testing.T) {
	provider, _ := anthropicProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	})

	req, _ := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[]}`))
	rec := httptest.NewRecorder()
	got, err := newUpstreamClient().forward(context.Background(), provider, types.Model{}, req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if got.StatusSuccess {
		t.Error("a 429 was reported as success")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want the upstream 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_error") {
		t.Errorf("the provider's own reason was lost: %s", rec.Body.String())
	}
}

func TestAnthropicSendsItsAuthAndVersionHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_, _ = io.WriteString(w, `{"id":"m","content":[],"usage":{}}`)
	}))
	t.Cleanup(srv.Close)

	provider := types.Provider{
		Kind:   types.ProviderKindAnthropic,
		Config: map[string]any{"base_url": srv.URL, "api_key": "sk-ant-test"},
	}
	req, _ := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[]}`))
	if _, err := newUpstreamClient().forward(context.Background(), provider, types.Model{}, req, httptest.NewRecorder()); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if gotHeaders.Get("x-api-key") != "sk-ant-test" {
		t.Errorf("x-api-key = %q", gotHeaders.Get("x-api-key"))
	}
	if gotHeaders.Get("anthropic-version") != anthropicVersion {
		t.Errorf("anthropic-version = %q", gotHeaders.Get("anthropic-version"))
	}
	if gotHeaders.Get("Authorization") != "" {
		t.Error("a bearer token was sent to Anthropic, which does not use one")
	}
}
