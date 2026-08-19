package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// AWS Bedrock, over the Converse API.
//
// Converse is the reason this adapter is one file rather than one per
// model family: it is Bedrock's own normalisation across Anthropic,
// Llama, Mistral and the rest, so a single translation reaches every
// model in the catalogue. What it costs in exchange is SigV4 signing
// (aws_sigv4.go) and a binary event-stream framing for the streaming
// case (aws_eventstream.go) instead of SSE.
//
// The shape is close to Anthropic's -- system prompt outside the turns,
// tool calls and results as content blocks -- which is not a
// coincidence, but the field names all differ and the streaming wire
// format is not SSE at all.

const bedrockService = "bedrock"

type bedrockRequest struct {
	Messages        []bedrockMessage     `json:"messages"`
	System          []bedrockSystemBlock `json:"system,omitempty"`
	InferenceConfig *bedrockInference    `json:"inferenceConfig,omitempty"`
	ToolConfig      *bedrockToolConfig   `json:"toolConfig,omitempty"`
}

type bedrockSystemBlock struct {
	Text string `json:"text"`
}

type bedrockMessage struct {
	Role    string         `json:"role"`
	Content []bedrockBlock `json:"content"`
}

// bedrockBlock is the union of content block shapes. Exactly one field
// is set per block.
type bedrockBlock struct {
	Text       string             `json:"text,omitempty"`
	ToolUse    *bedrockToolUse    `json:"toolUse,omitempty"`
	ToolResult *bedrockToolResult `json:"toolResult,omitempty"`
}

type bedrockToolUse struct {
	ToolUseID string          `json:"toolUseId"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
}

type bedrockToolResult struct {
	ToolUseID string         `json:"toolUseId"`
	Content   []bedrockBlock `json:"content"`
	Status    string         `json:"status,omitempty"`
}

type bedrockInference struct {
	MaxTokens     int      `json:"maxTokens,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	TopP          *float64 `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

type bedrockToolConfig struct {
	Tools      []bedrockToolEntry `json:"tools"`
	ToolChoice json.RawMessage    `json:"toolChoice,omitempty"`
}

type bedrockToolEntry struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolSpec struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	InputSchema bedrockInputSchema `json:"inputSchema"`
}

type bedrockInputSchema struct {
	JSON json.RawMessage `json:"json"`
}

func bedrockBody(req chatRequest) ([]byte, error) {
	out := bedrockRequest{}

	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			if m.Content != "" {
				out.System = append(out.System, bedrockSystemBlock{Text: m.Content})
			}

		case "tool":
			id, _ := jsonString(m.raw["tool_call_id"])
			out.Messages = append(out.Messages, bedrockMessage{
				Role: "user",
				Content: []bedrockBlock{{ToolResult: &bedrockToolResult{
					ToolUseID: id,
					Content:   []bedrockBlock{{Text: m.Content}},
					Status:    "success",
				}}},
			})

		case "assistant":
			blocks, err := bedrockAssistantBlocks(m)
			if err != nil {
				return nil, err
			}
			if len(blocks) == 0 {
				continue
			}
			out.Messages = append(out.Messages, bedrockMessage{Role: "assistant", Content: blocks})

		default:
			out.Messages = append(out.Messages, bedrockMessage{
				Role:    "user",
				Content: []bedrockBlock{{Text: m.Content}},
			})
		}
	}

	cfg := bedrockInference{MaxTokens: req.MaxTokens}
	if err := decodeField(req.raw, "temperature", &cfg.Temperature); err != nil {
		return nil, err
	}
	if err := decodeField(req.raw, "top_p", &cfg.TopP); err != nil {
		return nil, err
	}
	if err := decodeStopSequences(req.raw, &cfg.StopSequences); err != nil {
		return nil, err
	}
	if cfg.MaxTokens > 0 || cfg.Temperature != nil || cfg.TopP != nil || len(cfg.StopSequences) > 0 {
		out.InferenceConfig = &cfg
	}

	if err := bedrockTools(req.raw, &out); err != nil {
		return nil, err
	}

	return json.Marshal(out)
}

func bedrockAssistantBlocks(m chatMessage) ([]bedrockBlock, error) {
	var blocks []bedrockBlock
	if m.Content != "" {
		blocks = append(blocks, bedrockBlock{Text: m.Content})
	}

	raw, ok := m.raw["tool_calls"]
	if !ok || string(raw) == "null" {
		return blocks, nil
	}
	var calls []openAIToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return nil, fmt.Errorf("gateway: bedrock: tool_calls: %w", err)
	}
	for _, call := range calls {
		args := call.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		blocks = append(blocks, bedrockBlock{ToolUse: &bedrockToolUse{
			ToolUseID: call.ID,
			Name:      call.Function.Name,
			Input:     json.RawMessage(args),
		}})
	}
	return blocks, nil
}

func bedrockTools(raw map[string]json.RawMessage, out *bedrockRequest) error {
	toolsRaw, ok := raw["tools"]
	if ok && string(toolsRaw) != "null" {
		var tools []struct {
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return fmt.Errorf("gateway: bedrock: tools: %w", err)
		}
		cfg := &bedrockToolConfig{}
		for _, t := range tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			cfg.Tools = append(cfg.Tools, bedrockToolEntry{ToolSpec: bedrockToolSpec{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: bedrockInputSchema{JSON: schema},
			}})
		}
		if len(cfg.Tools) > 0 {
			out.ToolConfig = cfg
		}
	}

	choiceRaw, ok := raw["tool_choice"]
	if !ok || string(choiceRaw) == "null" || out.ToolConfig == nil {
		return nil
	}
	if s, err := jsonString(choiceRaw); err == nil {
		switch s {
		case "auto":
			out.ToolConfig.ToolChoice = json.RawMessage(`{"auto":{}}`)
		case "required":
			out.ToolConfig.ToolChoice = json.RawMessage(`{"any":{}}`)
		case "none":
			// Converse has no "none"; withholding the tools says it.
			out.ToolConfig = nil
		}
		return nil
	}
	var named struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(choiceRaw, &named); err != nil {
		return fmt.Errorf("gateway: bedrock: tool_choice: %w", err)
	}
	if named.Function.Name != "" {
		out.ToolConfig.ToolChoice = json.RawMessage(fmt.Sprintf(`{"tool":{"name":%q}}`, named.Function.Name))
	}
	return nil
}

// -- response translation ---------------------------------------------------

type bedrockResponse struct {
	Output struct {
		Message bedrockMessage `json:"message"`
	} `json:"output"`
	StopReason string `json:"stopReason"`
	Usage      struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"usage"`
}

func mapBedrockStopReason(reason string, sawToolCall bool) string {
	if sawToolCall {
		return "tool_calls"
	}
	switch reason {
	case "max_tokens":
		return "length"
	case "content_filtered", "guardrail_intervened":
		return "content_filter"
	case "":
		return ""
	default:
		return "stop"
	}
}

func bedrockToOpenAI(raw []byte, model types.Model) ([]byte, *chatUsage, error) {
	var parsed bedrockResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("gateway: bedrock: decode response: %w", err)
	}

	var text strings.Builder
	var calls []openAIToolCall
	for _, block := range parsed.Output.Message.Content {
		if block.Text != "" {
			text.WriteString(block.Text)
		}
		if block.ToolUse == nil {
			continue
		}
		call := openAIToolCall{ID: block.ToolUse.ToolUseID, Type: "function"}
		call.Function.Name = block.ToolUse.Name
		call.Function.Arguments = string(block.ToolUse.Input)
		if call.Function.Arguments == "" {
			call.Function.Arguments = "{}"
		}
		calls = append(calls, call)
	}

	usage := &chatUsage{
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}

	message := map[string]any{"role": "assistant", "content": text.String()}
	if len(calls) > 0 {
		message["tool_calls"] = calls
		message["content"] = nil
	}

	body, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-bedrock",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model.ModelIdentifier,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": mapBedrockStopReason(parsed.StopReason, len(calls) > 0),
		}},
		"usage": map[string]any{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.PromptTokens + usage.CompletionTokens,
		},
	})
	return body, usage, err
}

// -- forwarding -------------------------------------------------------------

func (c *upstreamClient) forwardBedrock(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	region, _ := p.Config["region"].(string)
	if region == "" {
		return outcome{}, fmt.Errorf("gateway: provider %q has no region configured", p.Name)
	}
	accessKey, _ := p.Config["access_key_id"].(string)
	secretKey, _ := p.Config["secret_access_key"].(string)
	sessionToken, _ := p.Config["session_token"].(string)

	baseURL, _ := p.Config["base_url"].(string)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
	}

	action := "converse"
	if req.Stream {
		action = "converse-stream"
	}
	endpoint := fmt.Sprintf("%s/model/%s/%s", baseURL, url.PathEscape(model.ModelIdentifier), action)

	body, err := bedrockBody(req)
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	signV4(upstreamReq, body, accessKey, secretKey, sessionToken, region, bedrockService, time.Now())

	resp, err := c.httpClient.Do(upstreamReq)
	if err != nil {
		return outcome{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return relayUpstreamError(w, resp)
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		usage, err := relayBedrockStream(flushWriter{w}, resp.Body, model, req.wantsUsageFrame())
		if err != nil {
			return outcome{}, err
		}
		return outcome{StatusSuccess: true, Usage: usage}, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{}, err
	}
	translated, usage, err := bedrockToOpenAI(raw, model)
	if err != nil {
		return outcome{}, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(translated); err != nil {
		return outcome{}, err
	}
	return outcome{StatusSuccess: true, Usage: usage}, nil
}
