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

func geminiTranslate(t *testing.T, clientJSON string) geminiRequest {
	t.Helper()
	req, err := decodeChatRequest(strings.NewReader(clientJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := geminiBody(req)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var out geminiRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	return out
}

func TestGeminiMapsRolesAndHoistsSystemInstruction(t *testing.T) {
	got := geminiTranslate(t, `{"model":"m","messages":[
		{"role":"system","content":"be terse"},
		{"role":"user","content":"hi"},
		{"role":"assistant","content":"hello"}]}`)

	if got.SystemInstruction == nil || got.SystemInstruction.Parts[0].Text != "be terse" {
		t.Errorf("systemInstruction = %+v", got.SystemInstruction)
	}
	if len(got.Contents) != 2 {
		t.Fatalf("contents = %+v", got.Contents)
	}
	if got.Contents[0].Role != "user" || got.Contents[1].Role != "model" {
		t.Errorf("roles = %q, %q; want user, model", got.Contents[0].Role, got.Contents[1].Role)
	}
}

// The old translation dropped tools entirely, so function calling looked
// broken rather than unsupported.
func TestGeminiDeclaresTools(t *testing.T) {
	got := geminiTranslate(t, `{"model":"m","messages":[],
		"tools":[{"type":"function","function":{
			"name":"get_weather","description":"look it up",
			"parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]}`)

	if len(got.Tools) != 1 || len(got.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools = %+v", got.Tools)
	}
	decl := got.Tools[0].FunctionDeclarations[0]
	if decl.Name != "get_weather" || decl.Description != "look it up" {
		t.Errorf("declaration = %+v", decl)
	}
	if !strings.Contains(string(decl.Parameters), `"city"`) {
		t.Errorf("parameters = %s", decl.Parameters)
	}
}

// Gemini matches a tool result to its call by function name; OpenAI
// matches by call id. The id has to be resolved back to a name from the
// assistant turn that made the call.
func TestGeminiResolvesToolResultsBackToFunctionNames(t *testing.T) {
	got := geminiTranslate(t, `{"model":"m","messages":[
		{"role":"user","content":"weather?"},
		{"role":"assistant","content":null,"tool_calls":[
			{"id":"call_0","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]},
		{"role":"tool","tool_call_id":"call_0","content":"{\"tempC\":18}"}]}`)

	if len(got.Contents) != 3 {
		t.Fatalf("contents = %+v", got.Contents)
	}

	call := got.Contents[1].Parts[0].FunctionCall
	if call == nil || call.Name != "get_weather" {
		t.Fatalf("functionCall = %+v", got.Contents[1].Parts[0])
	}
	if string(call.Args) != `{"city":"SF"}` {
		t.Errorf("args = %s", call.Args)
	}

	response := got.Contents[2].Parts[0].FunctionResponse
	if response == nil {
		t.Fatalf("functionResponse missing: %+v", got.Contents[2])
	}
	if response.Name != "get_weather" {
		t.Errorf("functionResponse.name = %q, want the id resolved to a name", response.Name)
	}
	if string(response.Response) != `{"tempC":18}` {
		t.Errorf("response = %s", response.Response)
	}
}

// Gemini wants an object in functionResponse; a tool that returned bare
// text gets one built around it rather than being rejected.
func TestGeminiWrapsANonObjectToolResult(t *testing.T) {
	got := geminiTranslate(t, `{"model":"m","messages":[
		{"role":"assistant","content":null,"tool_calls":[
			{"id":"c","type":"function","function":{"name":"f","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"c","content":"18 degrees"}]}`)

	response := got.Contents[1].Parts[0].FunctionResponse
	if response == nil || string(response.Response) != `{"result":"18 degrees"}` {
		t.Errorf("response = %+v", response)
	}
}

func TestGeminiMapsToolChoice(t *testing.T) {
	base := `{"model":"m","messages":[],"tools":[{"type":"function","function":{"name":"f"}}],`

	for choice, want := range map[string]string{
		`"auto"`:     "AUTO",
		`"required"`: "ANY",
		`"none"`:     "NONE",
	} {
		got := geminiTranslate(t, base+`"tool_choice":`+choice+`}`)
		if got.ToolConfig == nil || got.ToolConfig.FunctionCallingConfig.Mode != want {
			t.Errorf("tool_choice %s -> %+v, want mode %s", choice, got.ToolConfig, want)
		}
	}

	named := geminiTranslate(t, base+`"tool_choice":{"type":"function","function":{"name":"f"}}}`)
	if named.ToolConfig == nil || named.ToolConfig.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("named choice -> %+v", named.ToolConfig)
	}
	if allowed := named.ToolConfig.FunctionCallingConfig.AllowedFunctionNames; len(allowed) != 1 || allowed[0] != "f" {
		t.Errorf("allowedFunctionNames = %v", allowed)
	}
}

func TestGeminiCarriesGenerationConfig(t *testing.T) {
	got := geminiTranslate(t, `{"model":"m","messages":[],"temperature":0.4,"top_p":0.7,"max_tokens":128,"stop":["END"]}`)
	if got.GenerationConfig == nil {
		t.Fatal("generationConfig missing")
	}
	if *got.GenerationConfig.Temperature != 0.4 || *got.GenerationConfig.TopP != 0.7 {
		t.Errorf("sampling = %+v", got.GenerationConfig)
	}
	if got.GenerationConfig.MaxOutputTokens != 128 {
		t.Errorf("maxOutputTokens = %d", got.GenerationConfig.MaxOutputTokens)
	}
	if len(got.GenerationConfig.StopSequences) != 1 {
		t.Errorf("stopSequences = %v", got.GenerationConfig.StopSequences)
	}
}

func TestGeminiResponseBecomesOpenAIShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "goog-test" {
			t.Errorf("api key header = %q", r.Header.Get("x-goog-api-key"))
		}
		if strings.Contains(r.URL.RawQuery, "key=") {
			t.Error("the credential was put in the query string")
		}
		_, _ = io.WriteString(w, `{
			"candidates":[{"content":{"role":"model","parts":[
				{"text":"checking"},
				{"functionCall":{"name":"get_weather","args":{"city":"SF"}}}]},
			  "finishReason":"STOP"}],
			"usageMetadata":{"promptTokenCount":31,"candidatesTokenCount":12}}`)
	}))
	t.Cleanup(srv.Close)

	provider := types.Provider{
		Kind:   types.ProviderKindGemini,
		Config: map[string]any{"base_url": srv.URL, "api_key": "goog-test"},
	}
	req, _ := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()

	got, err := newUpstreamClient().forward(context.Background(), provider, types.Model{ModelIdentifier: "gemini-2.5-pro"}, req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if got.Usage == nil || got.Usage.PromptTokens != 31 || got.Usage.CompletionTokens != 12 {
		t.Errorf("usage = %+v, want 31/12", got.Usage)
	}

	var out struct {
		Object  string `json:"object"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not OpenAI-shaped: %v\n%s", err, rec.Body.String())
	}
	if out.Object != "chat.completion" {
		t.Errorf("object = %q", out.Object)
	}
	// A candidate that called a function finishes as tool_calls whatever
	// Gemini's own finishReason said.
	if out.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %q", out.Choices[0].FinishReason)
	}
	calls := out.Choices[0].Message.ToolCalls
	if len(calls) != 1 || calls[0].Function.Name != "get_weather" {
		t.Fatalf("tool_calls = %+v", calls)
	}
	if calls[0].ID == "" {
		t.Error("no id synthesised for a Gemini function call, so the caller cannot answer it")
	}
}

func TestGeminiStreamBecomesOpenAIChunks(t *testing.T) {
	stream := "data: " + `{"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}` + "\n\n" +
		"data: " + `{"candidates":[{"content":{"parts":[{"text":"lo"}]}}]}` + "\n\n" +
		"data: " + `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"f","args":{"a":1}}}]},"finishReason":"STOP"}],` +
		`"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":8}}` + "\n\n"

	var out strings.Builder
	usage, err := relayGeminiStream(&out, strings.NewReader(stream), types.Model{ModelIdentifier: "gemini"}, true)
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 8 {
		t.Errorf("usage = %+v", usage)
	}

	chunks := openAIChunks(t, out.String())
	var text strings.Builder
	sawToolCall := false
	for _, c := range chunks {
		if s, ok := delta(t, c)["content"].(string); ok {
			text.WriteString(s)
		}
		if _, ok := delta(t, c)["tool_calls"]; ok {
			sawToolCall = true
		}
	}
	if text.String() != "Hello" {
		t.Errorf("reassembled text = %q", text.String())
	}
	if !sawToolCall {
		t.Error("the function call never reached the caller")
	}
	if !strings.Contains(out.String(), `"prompt_tokens":5`) {
		t.Error("no usage frame for a caller who asked")
	}
	if !strings.HasSuffix(out.String(), "data: [DONE]\n\n") {
		t.Error("stream did not end with [DONE]")
	}
}
