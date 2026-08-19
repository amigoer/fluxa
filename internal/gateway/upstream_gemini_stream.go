package gateway

import (
	"encoding/json"
	"io"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// Gemini's streaming shape translated into OpenAI's.
//
// Simpler than Anthropic's: with alt=sse every frame is a whole
// generateContent response carrying the newest parts, so there is no
// content-block state machine to keep. Tool calls arrive complete rather
// than as JSON fragments, which means each one becomes a single OpenAI
// tool_call delta with its arguments already whole.
func relayGeminiStream(dst io.Writer, src io.Reader, model types.Model, emitUsageFrame bool) (*chatUsage, error) {
	created := time.Now().Unix()
	usage := chatUsage{}
	finishReason := ""
	toolCallsSeen := 0

	emit := func(delta any, finish any) error {
		payload, err := json.Marshal(map[string]any{
			"id":      "chatcmpl-gemini",
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

	if err := emit(map[string]any{"role": "assistant", "content": ""}, nil); err != nil {
		return nil, err
	}

	err := scanSSEEvents(src, func(_, data string) error {
		var frame geminiResponse
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			return nil
		}

		if frame.UsageMetadata.PromptTokenCount > 0 {
			usage.PromptTokens = frame.UsageMetadata.PromptTokenCount
		}
		if frame.UsageMetadata.CandidatesTokenCount > 0 {
			usage.CompletionTokens = frame.UsageMetadata.CandidatesTokenCount
		}
		if len(frame.Candidates) == 0 {
			return nil
		}

		candidate := frame.Candidates[0]
		text, calls := geminiParts(candidate.Content.Parts, "call")

		if text != "" {
			if err := emit(map[string]any{"content": text}, nil); err != nil {
				return err
			}
		}
		for _, call := range calls {
			// The index is assigned across the whole stream, not within
			// one frame -- an OpenAI client keys its accumulation on it.
			index := toolCallsSeen
			toolCallsSeen++
			if err := emit(map[string]any{
				"tool_calls": []any{map[string]any{
					"index": index,
					"id":    call.ID,
					"type":  "function",
					"function": map[string]any{
						"name":      call.Function.Name,
						"arguments": call.Function.Arguments,
					},
				}},
			}, nil); err != nil {
				return err
			}
		}
		if candidate.FinishReason != "" {
			finishReason = mapGeminiFinishReason(candidate.FinishReason, toolCallsSeen > 0)
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
			"id":      "chatcmpl-gemini",
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
