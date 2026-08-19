package gateway

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// awsFrame encodes one event-stream frame the way Bedrock writes them,
// so the decoder is tested against the real framing rather than a
// convenient stand-in.
func awsFrame(eventType string, payload string) []byte {
	return awsFrameOfType("event", eventType, payload)
}

func awsFrameOfType(messageType, eventType, payload string) []byte {
	header := func(name, value string) []byte {
		var b bytes.Buffer
		b.WriteByte(byte(len(name)))
		b.WriteString(name)
		b.WriteByte(7) // string
		_ = binary.Write(&b, binary.BigEndian, uint16(len(value)))
		b.WriteString(value)
		return b.Bytes()
	}

	var headers bytes.Buffer
	headers.Write(header(":message-type", messageType))
	headers.Write(header(":event-type", eventType))

	total := uint32(12 + headers.Len() + len(payload) + 4)
	var frame bytes.Buffer
	_ = binary.Write(&frame, binary.BigEndian, total)
	_ = binary.Write(&frame, binary.BigEndian, uint32(headers.Len()))
	_ = binary.Write(&frame, binary.BigEndian, uint32(0)) // prelude CRC, unchecked
	frame.Write(headers.Bytes())
	frame.WriteString(payload)
	_ = binary.Write(&frame, binary.BigEndian, uint32(0)) // message CRC, unchecked
	return frame.Bytes()
}

func bedrockTranslate(t *testing.T, clientJSON string) bedrockRequest {
	t.Helper()
	req, err := decodeChatRequest(strings.NewReader(clientJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := bedrockBody(req)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var out bedrockRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	return out
}

func TestBedrockTranslatesAToolCallingConversation(t *testing.T) {
	got := bedrockTranslate(t, `{
		"model":"m",
		"messages":[
			{"role":"system","content":"be terse"},
			{"role":"user","content":"weather?"},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"tu_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]},
			{"role":"tool","tool_call_id":"tu_1","content":"18C"}],
		"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}],
		"tool_choice":"required"}`)

	if len(got.System) != 1 || got.System[0].Text != "be terse" {
		t.Errorf("system = %+v", got.System)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %+v", got.Messages)
	}

	use := got.Messages[1].Content[0].ToolUse
	if use == nil || use.ToolUseID != "tu_1" || use.Name != "get_weather" {
		t.Fatalf("toolUse = %+v", got.Messages[1].Content[0])
	}
	if string(use.Input) != `{"city":"SF"}` {
		t.Errorf("toolUse input = %s", use.Input)
	}

	result := got.Messages[2].Content[0].ToolResult
	if result == nil || result.ToolUseID != "tu_1" {
		t.Fatalf("toolResult = %+v", got.Messages[2].Content[0])
	}
	if len(result.Content) != 1 || result.Content[0].Text != "18C" {
		t.Errorf("toolResult content = %+v", result.Content)
	}

	if got.ToolConfig == nil || len(got.ToolConfig.Tools) != 1 {
		t.Fatalf("toolConfig = %+v", got.ToolConfig)
	}
	if got.ToolConfig.Tools[0].ToolSpec.Name != "get_weather" {
		t.Errorf("toolSpec = %+v", got.ToolConfig.Tools[0].ToolSpec)
	}
	if string(got.ToolConfig.ToolChoice) != `{"any":{}}` {
		t.Errorf("toolChoice = %s", got.ToolConfig.ToolChoice)
	}
}

func TestBedrockCarriesInferenceConfig(t *testing.T) {
	got := bedrockTranslate(t, `{"model":"m","messages":[],"temperature":0.5,"max_tokens":64,"stop":["END"]}`)
	if got.InferenceConfig == nil {
		t.Fatal("inferenceConfig missing")
	}
	if got.InferenceConfig.MaxTokens != 64 || *got.InferenceConfig.Temperature != 0.5 {
		t.Errorf("inferenceConfig = %+v", got.InferenceConfig)
	}
	if len(got.InferenceConfig.StopSequences) != 1 {
		t.Errorf("stopSequences = %v", got.InferenceConfig.StopSequences)
	}
}

func TestBedrockResponseBecomesOpenAIShape(t *testing.T) {
	var signedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signedAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{
			"output":{"message":{"role":"assistant","content":[
				{"text":"it is 18C"}]}},
			"stopReason":"end_turn",
			"usage":{"inputTokens":19,"outputTokens":6}}`)
	}))
	t.Cleanup(srv.Close)

	provider := types.Provider{
		Kind: types.ProviderKindBedrock,
		Config: map[string]any{
			"base_url":          srv.URL,
			"region":            "us-east-1",
			"access_key_id":     "AKID",
			"secret_access_key": "secret",
		},
	}
	req, _ := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()

	got, err := newUpstreamClient().forward(context.Background(), provider,
		types.Model{ModelIdentifier: "anthropic.claude-sonnet-4-v1:0"}, req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if !strings.HasPrefix(signedAuth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("request was not SigV4 signed: %q", signedAuth)
	}
	if got.Usage == nil || got.Usage.PromptTokens != 19 || got.Usage.CompletionTokens != 6 {
		t.Errorf("usage = %+v, want 19/6", got.Usage)
	}

	var out struct {
		Object  string `json:"object"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not OpenAI-shaped: %v\n%s", err, rec.Body.String())
	}
	if out.Object != "chat.completion" || out.Choices[0].Message.Content != "it is 18C" {
		t.Errorf("response = %+v", out)
	}
	if out.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q", out.Choices[0].FinishReason)
	}
}

// Bedrock streams binary frames, not SSE. This exercises the decoder and
// the translation together.
func TestBedrockBinaryStreamBecomesOpenAIChunks(t *testing.T) {
	var stream bytes.Buffer
	stream.Write(awsFrame("messageStart", `{"role":"assistant"}`))
	stream.Write(awsFrame("contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"Hel"}}`))
	stream.Write(awsFrame("contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"lo"}}`))
	stream.Write(awsFrame("contentBlockStart", `{"contentBlockIndex":1,"start":{"toolUse":{"toolUseId":"tu_1","name":"get_weather"}}}`))
	stream.Write(awsFrame("contentBlockDelta", `{"contentBlockIndex":1,"delta":{"toolUse":{"input":"{\"city\":"}}}`))
	stream.Write(awsFrame("contentBlockDelta", `{"contentBlockIndex":1,"delta":{"toolUse":{"input":"\"SF\"}"}}}`))
	stream.Write(awsFrame("messageStop", `{"stopReason":"tool_use"}`))
	stream.Write(awsFrame("metadata", `{"usage":{"inputTokens":40,"outputTokens":15}}`))

	var out strings.Builder
	usage, err := relayBedrockStream(&out, bytes.NewReader(stream.Bytes()), types.Model{ModelIdentifier: "m"}, false)
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if usage.PromptTokens != 40 || usage.CompletionTokens != 15 {
		t.Errorf("usage = %+v, want 40/15", usage)
	}

	chunks := openAIChunks(t, out.String())
	var text, args strings.Builder
	toolName, toolID := "", ""
	for _, c := range chunks {
		if s, ok := delta(t, c)["content"].(string); ok {
			text.WriteString(s)
		}
		calls, ok := delta(t, c)["tool_calls"].([]any)
		if !ok {
			continue
		}
		call, _ := calls[0].(map[string]any)
		if id, ok := call["id"].(string); ok && id != "" {
			toolID = id
		}
		fn, _ := call["function"].(map[string]any)
		if n, ok := fn["name"].(string); ok && n != "" {
			toolName = n
		}
		if a, ok := fn["arguments"].(string); ok {
			args.WriteString(a)
		}
	}

	if text.String() != "Hello" {
		t.Errorf("reassembled text = %q", text.String())
	}
	if toolID != "tu_1" || toolName != "get_weather" {
		t.Errorf("tool call = %s/%s", toolID, toolName)
	}
	if args.String() != `{"city":"SF"}` {
		t.Errorf("arguments = %q, want the fragments joined", args.String())
	}

	last := chunks[len(chunks)-1]
	choices, _ := last["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v", choice["finish_reason"])
	}
}

// A stream that fails partway has already sent its headers. Reporting a
// clean finish would tell the caller the answer is complete.
func TestBedrockStreamSurfacesAnException(t *testing.T) {
	var stream bytes.Buffer
	stream.Write(awsFrame("messageStart", `{"role":"assistant"}`))
	stream.Write(awsFrameOfType("exception", "throttlingException", `{"message":"too many requests"}`))

	var out strings.Builder
	_, err := relayBedrockStream(&out, bytes.NewReader(stream.Bytes()), types.Model{}, false)
	if err == nil {
		t.Fatal("a mid-stream exception was reported as a clean finish")
	}
	if !strings.Contains(err.Error(), "throttlingException") {
		t.Errorf("err = %v, want the exception named", err)
	}
}

// A header type the decoder has not seen must be skipped by its real
// width, not fail the whole stream.
func TestEventStreamSkipsNonStringHeaders(t *testing.T) {
	var headers bytes.Buffer
	headers.WriteByte(byte(len(":event-type")))
	headers.WriteString(":event-type")
	headers.WriteByte(7)
	_ = binary.Write(&headers, binary.BigEndian, uint16(len("metadata")))
	headers.WriteString("metadata")
	// an 8-byte timestamp header AWS might add
	headers.WriteByte(byte(len(":timestamp")))
	headers.WriteString(":timestamp")
	headers.WriteByte(8)
	_ = binary.Write(&headers, binary.BigEndian, int64(1))

	payload := `{"usage":{"inputTokens":3,"outputTokens":4}}`
	total := uint32(12 + headers.Len() + len(payload) + 4)
	var frame bytes.Buffer
	_ = binary.Write(&frame, binary.BigEndian, total)
	_ = binary.Write(&frame, binary.BigEndian, uint32(headers.Len()))
	_ = binary.Write(&frame, binary.BigEndian, uint32(0))
	frame.Write(headers.Bytes())
	frame.WriteString(payload)
	_ = binary.Write(&frame, binary.BigEndian, uint32(0))

	var seen []eventFrame
	if err := readEventStream(bytes.NewReader(frame.Bytes()), func(f eventFrame) error {
		seen = append(seen, f)
		return nil
	}); err != nil {
		t.Fatalf("a timestamp header killed the stream: %v", err)
	}
	if len(seen) != 1 || seen[0].EventType != "metadata" {
		t.Errorf("frames = %+v", seen)
	}
}
