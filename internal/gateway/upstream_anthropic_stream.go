package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// Anthropic's streaming shape translated into OpenAI's.
//
// The two disagree about what a stream is. Anthropic sends typed events
// over indexed content blocks -- a block opens, deltas arrive against
// its index, the block closes -- so a tool call's name arrives once at
// the start and its arguments accumulate as JSON fragments afterwards.
// OpenAI sends one flat list of deltas where each tool call carries its
// own index inside the delta. Translating means holding just enough
// state to know which block is open and, for tool calls, what position
// it occupies in the array OpenAI expects.

// anthropicStreamEvent is the envelope every event shares. Fields not
// carried by a given event type stay zero.
type anthropicStreamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicStreamState is what has to be remembered between events.
type anthropicStreamState struct {
	id    string
	model string

	// toolCallIndex maps an Anthropic content-block index to the
	// position that tool call occupies in OpenAI's tool_calls array.
	// They differ because text blocks take up block indexes without
	// being tool calls.
	toolCallIndex map[int]int
	nextToolCall  int

	finishReason string
	usage        chatUsage
}

// relayAnthropicStream reads an Anthropic SSE stream and writes the
// equivalent OpenAI one to dst, returning the usage it reported.
//
// emitUsageFrame mirrors the OpenAI path: the caller only gets a
// usage-carrying frame if it asked for one, but usage is read either
// way because the call is billed from it.
func relayAnthropicStream(dst io.Writer, src io.Reader, model types.Model, emitUsageFrame bool) (*chatUsage, error) {
	state := &anthropicStreamState{toolCallIndex: map[int]int{}}
	created := time.Now().Unix()

	emit := func(delta any, finishReason any) error {
		chunk := map[string]any{
			"id":      state.id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelNameFor(state.model, model),
			"choices": []any{map[string]any{
				"index":         0,
				"delta":         delta,
				"finish_reason": finishReason,
			}},
		}
		payload, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		return writeSSEData(dst, payload)
	}

	err := scanSSEEvents(src, func(_, data string) error {
		var ev anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			// Not a shape this understands. Anthropic sends "ping" and
			// may add event types later; neither is a reason to fail a
			// call that is otherwise working.
			return nil
		}

		switch ev.Type {
		case "message_start":
			state.id = ev.Message.ID
			state.model = ev.Message.Model
			state.usage.PromptTokens = ev.Message.Usage.InputTokens
			state.usage.CompletionTokens = ev.Message.Usage.OutputTokens
			return emit(map[string]any{"role": "assistant", "content": ""}, nil)

		case "content_block_start":
			if ev.ContentBlock.Type != "tool_use" {
				return nil
			}
			index := state.nextToolCall
			state.toolCallIndex[ev.Index] = index
			state.nextToolCall++
			return emit(map[string]any{
				"tool_calls": []any{map[string]any{
					"index": index,
					"id":    ev.ContentBlock.ID,
					"type":  "function",
					"function": map[string]any{
						"name":      ev.ContentBlock.Name,
						"arguments": "",
					},
				}},
			}, nil)

		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text == "" {
					return nil
				}
				return emit(map[string]any{"content": ev.Delta.Text}, nil)
			case "input_json_delta":
				index, ok := state.toolCallIndex[ev.Index]
				if !ok {
					return nil
				}
				return emit(map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    index,
						"function": map[string]any{"arguments": ev.Delta.PartialJSON},
					}},
				}, nil)
			}
			return nil

		case "message_delta":
			if ev.Delta.StopReason != "" {
				state.finishReason = mapAnthropicStopReason(ev.Delta.StopReason)
			}
			if ev.Usage.OutputTokens > 0 {
				state.usage.CompletionTokens = ev.Usage.OutputTokens
			}
			return nil

		case "error":
			// The stream failed partway. Headers are long gone, so the
			// only honest thing left is to stop and let the caller see a
			// truncated stream rather than a fabricated clean ending.
			return fmt.Errorf("gateway: anthropic stream error: %s", data)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	finishReason := state.finishReason
	if finishReason == "" {
		finishReason = "stop"
	}
	if err := emit(map[string]any{}, finishReason); err != nil {
		return nil, err
	}

	if emitUsageFrame {
		payload, err := json.Marshal(map[string]any{
			"id":      state.id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelNameFor(state.model, model),
			"choices": []any{},
			"usage": map[string]any{
				"prompt_tokens":     state.usage.PromptTokens,
				"completion_tokens": state.usage.CompletionTokens,
				"total_tokens":      state.usage.PromptTokens + state.usage.CompletionTokens,
			},
		})
		if err != nil {
			return nil, err
		}
		if err := writeSSEData(dst, payload); err != nil {
			return nil, err
		}
	}

	if err := writeSSEData(dst, []byte(doneMarker)); err != nil {
		return nil, err
	}

	usage := state.usage
	return &usage, nil
}
