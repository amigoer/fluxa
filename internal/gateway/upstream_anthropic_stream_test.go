package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// anthropicSSE builds a stream the way Anthropic writes one: a named
// event plus its data payload.
func anthropicSSE(events ...[2]string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("event: " + e[0] + "\ndata: " + e[1] + "\n\n")
	}
	return b.String()
}

// openAIChunks parses the OpenAI stream the relay produced back into
// its chunks, dropping [DONE].
func openAIChunks(t *testing.T, stream string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(stream, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("chunk is not JSON: %v\n%s", err, payload)
		}
		out = append(out, chunk)
	}
	return out
}

// delta digs the delta object out of a chunk.
func delta(t *testing.T, chunk map[string]any) map[string]any {
	t.Helper()
	choices, _ := chunk["choices"].([]any)
	if len(choices) == 0 {
		return nil
	}
	choice, _ := choices[0].(map[string]any)
	d, _ := choice["delta"].(map[string]any)
	return d
}

func TestAnthropicStreamBecomesOpenAIChunks(t *testing.T) {
	stream := anthropicSSE(
		[2]string{"message_start", `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":25,"output_tokens":1}}}`},
		[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		[2]string{"ping", `{"type":"ping"}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}`},
		[2]string{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		[2]string{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	)

	var out strings.Builder
	usage, err := relayAnthropicStream(&out, strings.NewReader(stream), types.Model{ModelIdentifier: "m"}, false)
	if err != nil {
		t.Fatalf("relay: %v", err)
	}

	if usage == nil || usage.PromptTokens != 25 || usage.CompletionTokens != 9 {
		t.Errorf("usage = %+v, want 25/9", usage)
	}
	if !strings.HasSuffix(out.String(), "data: [DONE]\n\n") {
		t.Errorf("stream did not end with [DONE]:\n%s", out.String())
	}

	chunks := openAIChunks(t, out.String())
	if len(chunks) < 4 {
		t.Fatalf("got %d chunks:\n%s", len(chunks), out.String())
	}
	if chunks[0]["object"] != "chat.completion.chunk" {
		t.Errorf("object = %v", chunks[0]["object"])
	}
	if delta(t, chunks[0])["role"] != "assistant" {
		t.Errorf("first chunk delta = %v, want the assistant role", delta(t, chunks[0]))
	}

	var text strings.Builder
	for _, c := range chunks {
		if s, ok := delta(t, c)["content"].(string); ok {
			text.WriteString(s)
		}
	}
	if text.String() != "Hello there" {
		t.Errorf("reassembled text = %q, want %q", text.String(), "Hello there")
	}

	last := chunks[len(chunks)-1]
	choices, _ := last["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
}

// The hard part: Anthropic streams a tool call's name once when the
// block opens and its arguments as JSON fragments afterwards, indexed by
// content block. OpenAI wants an index into the tool_calls array, which
// is a different number because text blocks take up block indexes too.
func TestAnthropicStreamRebuildsToolCalls(t *testing.T) {
	stream := anthropicSSE(
		[2]string{"message_start", `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":10}}}`},
		// block 0 is text, so the first tool call is block 1 but tool_calls index 0
		[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"checking"}}`},
		[2]string{"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"get_weather"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"SF\"}"}}`},
		[2]string{"content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"tu_2","name":"get_time"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{}"}}`},
		[2]string{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":30}}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	)

	var out strings.Builder
	if _, err := relayAnthropicStream(&out, strings.NewReader(stream), types.Model{}, false); err != nil {
		t.Fatalf("relay: %v", err)
	}

	// Reassemble tool calls the way an OpenAI client would.
	names := map[float64]string{}
	ids := map[float64]string{}
	args := map[float64]*strings.Builder{}
	for _, c := range openAIChunks(t, out.String()) {
		calls, ok := delta(t, c)["tool_calls"].([]any)
		if !ok {
			continue
		}
		for _, raw := range calls {
			call, _ := raw.(map[string]any)
			idx, _ := call["index"].(float64)
			if args[idx] == nil {
				args[idx] = &strings.Builder{}
			}
			if id, ok := call["id"].(string); ok && id != "" {
				ids[idx] = id
			}
			fn, _ := call["function"].(map[string]any)
			if name, ok := fn["name"].(string); ok && name != "" {
				names[idx] = name
			}
			if a, ok := fn["arguments"].(string); ok {
				args[idx].WriteString(a)
			}
		}
	}

	if len(names) != 2 {
		t.Fatalf("rebuilt %d tool calls, want 2: %v", len(names), names)
	}
	if names[0] != "get_weather" || ids[0] != "tu_1" {
		t.Errorf("call 0 = %s/%s", ids[0], names[0])
	}
	if got := args[0].String(); got != `{"city":"SF"}` {
		t.Errorf("call 0 arguments = %q, want the fragments joined", got)
	}
	if names[1] != "get_time" || ids[1] != "tu_2" {
		t.Errorf("call 1 = %s/%s", ids[1], names[1])
	}
	if got := args[1].String(); got != `{}` {
		t.Errorf("call 1 arguments = %q", got)
	}

	chunks := openAIChunks(t, out.String())
	choices, _ := chunks[len(chunks)-1]["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", choice["finish_reason"])
	}
}

func TestAnthropicStreamEmitsAUsageFrameOnlyWhenAsked(t *testing.T) {
	stream := anthropicSSE(
		[2]string{"message_start", `{"type":"message_start","message":{"id":"m","usage":{"input_tokens":7}}}`},
		[2]string{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	)

	var without strings.Builder
	if _, err := relayAnthropicStream(&without, strings.NewReader(stream), types.Model{}, false); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if strings.Contains(without.String(), "prompt_tokens") {
		t.Errorf("a usage frame reached a caller who never asked:\n%s", without.String())
	}

	var with strings.Builder
	usage, err := relayAnthropicStream(&with, strings.NewReader(stream), types.Model{}, true)
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if !strings.Contains(with.String(), `"prompt_tokens":7`) {
		t.Errorf("no usage frame for a caller who asked:\n%s", with.String())
	}
	if usage.PromptTokens != 7 || usage.CompletionTokens != 3 {
		t.Errorf("usage = %+v", usage)
	}
}

// A stream that fails partway has already sent its headers. Fabricating
// a clean ending would tell the caller the answer is complete when it
// is not.
func TestAnthropicStreamSurfacesAMidStreamError(t *testing.T) {
	stream := anthropicSSE(
		[2]string{"message_start", `{"type":"message_start","message":{"id":"m","usage":{}}}`},
		[2]string{"error", `{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`},
	)

	var out strings.Builder
	if _, err := relayAnthropicStream(&out, strings.NewReader(stream), types.Model{}, false); err == nil {
		t.Fatal("a mid-stream error was reported as a clean finish")
	} else if !strings.Contains(err.Error(), "overloaded") {
		t.Errorf("err = %v, want the provider's own reason", err)
	}
}
