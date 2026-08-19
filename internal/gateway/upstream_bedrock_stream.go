package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// Bedrock's converse-stream translated into OpenAI's SSE.
//
// The events mirror Anthropic's -- blocks open, deltas arrive against an
// index, blocks close -- but they arrive as binary event-stream frames
// rather than SSE lines, and usage comes in a separate metadata event
// rather than riding on messageStop. The same index bookkeeping applies:
// a tool call's position in OpenAI's tool_calls array is not its Bedrock
// content-block index, because text blocks take up indexes too.

type bedrockStreamEvent struct {
	Start struct {
		ToolUse struct {
			ToolUseID string `json:"toolUseId"`
			Name      string `json:"name"`
		} `json:"toolUse"`
	} `json:"start"`
	Delta struct {
		Text    string `json:"text"`
		ToolUse struct {
			Input string `json:"input"`
		} `json:"toolUse"`
	} `json:"delta"`
	ContentBlockIndex int    `json:"contentBlockIndex"`
	StopReason        string `json:"stopReason"`
	Usage             struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"usage"`
}

func relayBedrockStream(dst io.Writer, src io.Reader, model types.Model, emitUsageFrame bool) (*chatUsage, error) {
	created := time.Now().Unix()
	usage := chatUsage{}
	finishReason := ""
	toolCallIndex := map[int]int{}
	nextToolCall := 0

	emit := func(delta any, finish any) error {
		payload, err := json.Marshal(map[string]any{
			"id":      "chatcmpl-bedrock",
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model.ModelIdentifier,
			"choices": []any{map[string]any{
				"index":         0,
				"delta":         delta,
				"finish_reason": finish,
			}},
		})
		if err != nil {
			return err
		}
		return writeSSEData(dst, payload)
	}

	err := readEventStream(src, func(frame eventFrame) error {
		if frame.MessageType == "exception" {
			// Headers are long gone by now. Stopping leaves the caller
			// with a truncated stream, which is the truth; a fabricated
			// clean ending would not be.
			return fmt.Errorf("gateway: bedrock stream exception %s: %s", frame.EventType, frame.Payload)
		}

		var ev bedrockStreamEvent
		if err := json.Unmarshal(frame.Payload, &ev); err != nil {
			return nil
		}

		switch frame.EventType {
		case "messageStart":
			return emit(map[string]any{"role": "assistant", "content": ""}, nil)

		case "contentBlockStart":
			if ev.Start.ToolUse.ToolUseID == "" {
				return nil
			}
			index := nextToolCall
			toolCallIndex[ev.ContentBlockIndex] = index
			nextToolCall++
			return emit(map[string]any{
				"tool_calls": []any{map[string]any{
					"index": index,
					"id":    ev.Start.ToolUse.ToolUseID,
					"type":  "function",
					"function": map[string]any{
						"name":      ev.Start.ToolUse.Name,
						"arguments": "",
					},
				}},
			}, nil)

		case "contentBlockDelta":
			if ev.Delta.Text != "" {
				return emit(map[string]any{"content": ev.Delta.Text}, nil)
			}
			if ev.Delta.ToolUse.Input == "" {
				return nil
			}
			index, ok := toolCallIndex[ev.ContentBlockIndex]
			if !ok {
				return nil
			}
			return emit(map[string]any{
				"tool_calls": []any{map[string]any{
					"index":    index,
					"function": map[string]any{"arguments": ev.Delta.ToolUse.Input},
				}},
			}, nil)

		case "messageStop":
			finishReason = mapBedrockStopReason(ev.StopReason, nextToolCall > 0)

		case "metadata":
			if ev.Usage.InputTokens > 0 {
				usage.PromptTokens = ev.Usage.InputTokens
			}
			if ev.Usage.OutputTokens > 0 {
				usage.CompletionTokens = ev.Usage.OutputTokens
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if finishReason == "" {
		finishReason = "stop"
	}
	if err := emit(map[string]any{}, finishReason); err != nil {
		return nil, err
	}

	if emitUsageFrame {
		payload, err := json.Marshal(map[string]any{
			"id":      "chatcmpl-bedrock",
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model.ModelIdentifier,
			"choices": []any{},
			"usage": map[string]any{
				"prompt_tokens":     usage.PromptTokens,
				"completion_tokens": usage.CompletionTokens,
				"total_tokens":      usage.PromptTokens + usage.CompletionTokens,
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
	return &usage, nil
}
