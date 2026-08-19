package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// Anthropic's Messages API, translated both ways so a caller holding an
// OpenAI client can reach Claude without changing anything.
//
// This translation did not exist before: the implementation removed in
// f5b192a proxied /v1/messages byte-for-byte for callers already
// speaking Anthropic, and answered /v1/chat/completions with 501. That
// is a different feature (and one worth having again -- see P1-1), not
// this one.
//
// What makes the two protocols genuinely different, rather than just
// differently spelled:
//
//   - the system prompt is a top-level field, not a message
//   - assistant tool calls and their results are content blocks inside
//     ordinary messages, not a parallel tool_calls array plus a "tool"
//     role
//   - max_tokens is required
//   - the stream is a sequence of typed events over a content-block
//     index, not a flat list of deltas

const (
	anthropicVersion = "2023-06-01"

	// anthropicDefaultMaxTokens is sent when the caller set no
	// max_tokens. OpenAI treats it as optional and Anthropic requires
	// it, so something has to be chosen; this matches what the routing
	// layer assumes for an uncapped request.
	anthropicDefaultMaxTokens = 4096
)

// -- request translation ----------------------------------------------------

type anthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	System        string             `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

// anthropicContent is the union of the block types this translation
// produces and reads. Only the fields relevant to a block's type are
// set, which is why every one is omitempty.
type anthropicContent struct {
	Type string `json:"type"`

	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// openAIToolCall is the assistant-side tool call as OpenAI models it,
// read off a message on the way in and written back on the way out.
type openAIToolCall struct {
	Index    *int   `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

// anthropicBody translates an OpenAI chat-completions request into an
// Anthropic Messages request.
func anthropicBody(req chatRequest, model types.Model) ([]byte, error) {
	out := anthropicRequest{
		Model:     model.ModelIdentifier,
		Stream:    req.Stream,
		MaxTokens: req.MaxTokens,
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = anthropicDefaultMaxTokens
	}

	if err := decodeField(req.raw, "temperature", &out.Temperature); err != nil {
		return nil, err
	}
	if err := decodeField(req.raw, "top_p", &out.TopP); err != nil {
		return nil, err
	}
	if err := decodeStopSequences(req.raw, &out.StopSequences); err != nil {
		return nil, err
	}
	if err := anthropicTools(req.raw, &out); err != nil {
		return nil, err
	}

	var systemParts []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			// Hoisted out of the message list: Anthropic carries the
			// system prompt beside the conversation, not inside it.
			if m.Content != "" {
				systemParts = append(systemParts, m.Content)
			}

		case "tool":
			// A tool result is a content block on a user turn here, and
			// it has to name the call it answers.
			toolUseID, _ := jsonString(m.raw["tool_call_id"])
			out.Messages = append(out.Messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: toolUseID,
					Content:   m.Content,
				}},
			})

		case "assistant":
			content, err := anthropicAssistantContent(m)
			if err != nil {
				return nil, err
			}
			if len(content) == 0 {
				continue
			}
			out.Messages = append(out.Messages, anthropicMessage{Role: "assistant", Content: content})

		default:
			out.Messages = append(out.Messages, anthropicMessage{
				Role:    "user",
				Content: []anthropicContent{{Type: "text", Text: m.Content}},
			})
		}
	}
	out.System = strings.Join(systemParts, "\n\n")

	return json.Marshal(out)
}

// anthropicAssistantContent turns an assistant turn -- text, tool calls,
// or both -- into content blocks.
func anthropicAssistantContent(m chatMessage) ([]anthropicContent, error) {
	var content []anthropicContent
	if m.Content != "" {
		content = append(content, anthropicContent{Type: "text", Text: m.Content})
	}

	raw, ok := m.raw["tool_calls"]
	if !ok || string(raw) == "null" {
		return content, nil
	}
	var calls []openAIToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return nil, fmt.Errorf("gateway: anthropic: tool_calls: %w", err)
	}
	for _, call := range calls {
		arguments := call.Function.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		content = append(content, anthropicContent{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: json.RawMessage(arguments),
		})
	}
	return content, nil
}

// anthropicTools translates OpenAI's tool declarations and tool_choice.
// Dropping them silently is what would make function calling look
// broken rather than unsupported, so a shape that cannot be translated
// is an error.
func anthropicTools(raw map[string]json.RawMessage, out *anthropicRequest) error {
	if toolsRaw, ok := raw["tools"]; ok && string(toolsRaw) != "null" {
		var tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return fmt.Errorf("gateway: anthropic: tools: %w", err)
		}
		for _, t := range tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			out.Tools = append(out.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: schema,
			})
		}
	}

	choiceRaw, ok := raw["tool_choice"]
	if !ok || string(choiceRaw) == "null" {
		return nil
	}
	// OpenAI spells it as a string ("auto"/"none"/"required") or an
	// object naming one function; Anthropic as an object either way.
	if s, err := jsonString(choiceRaw); err == nil {
		switch s {
		case "auto":
			out.ToolChoice = json.RawMessage(`{"type":"auto"}`)
		case "required":
			out.ToolChoice = json.RawMessage(`{"type":"any"}`)
		case "none":
			// Anthropic has no "none"; withholding the tools is the
			// equivalent, and says the same thing to the model.
			out.Tools = nil
		}
		return nil
	}
	var named struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(choiceRaw, &named); err != nil {
		return fmt.Errorf("gateway: anthropic: tool_choice: %w", err)
	}
	if named.Function.Name != "" {
		out.ToolChoice = json.RawMessage(fmt.Sprintf(`{"type":"tool","name":%q}`, named.Function.Name))
	}
	return nil
}

// decodeStopSequences accepts OpenAI's "stop", which is a string or an
// array of them.
func decodeStopSequences(raw map[string]json.RawMessage, out *[]string) error {
	v, ok := raw["stop"]
	if !ok || string(v) == "null" {
		return nil
	}
	if s, err := jsonString(v); err == nil {
		*out = []string{s}
		return nil
	}
	if err := json.Unmarshal(v, out); err != nil {
		return fmt.Errorf("gateway: anthropic: stop: %w", err)
	}
	return nil
}

func jsonString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

// -- response translation ---------------------------------------------------

type anthropicResponse struct {
	ID         string             `json:"id"`
	Model      string             `json:"model"`
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// mapAnthropicStopReason translates Anthropic's stop reasons to the
// finish_reason values an OpenAI client branches on.
func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "end_turn", "stop_sequence":
		return "stop"
	case "":
		return ""
	default:
		return "stop"
	}
}

// anthropicToOpenAI rebuilds a completed Anthropic response in the shape
// the caller's OpenAI client expects.
func anthropicToOpenAI(raw []byte, model types.Model) ([]byte, *chatUsage, error) {
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("gateway: anthropic: decode response: %w", err)
	}

	var text strings.Builder
	var toolCalls []openAIToolCall
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			call := openAIToolCall{ID: block.ID, Type: "function"}
			call.Function.Name = block.Name
			call.Function.Arguments = string(block.Input)
			if call.Function.Arguments == "" {
				call.Function.Arguments = "{}"
			}
			toolCalls = append(toolCalls, call)
		}
	}

	usage := &chatUsage{
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}

	message := map[string]any{"role": "assistant", "content": text.String()}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
		// OpenAI sends a null content alongside tool calls.
		message["content"] = nil
	}

	out := map[string]any{
		"id":      parsed.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelNameFor(parsed.Model, model),
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": mapAnthropicStopReason(parsed.StopReason),
		}},
		"usage": map[string]any{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.PromptTokens + usage.CompletionTokens,
		},
	}
	body, err := json.Marshal(out)
	return body, usage, err
}

// modelNameFor prefers what the provider called the model and falls back
// to what was configured, so the caller sees a real name either way.
func modelNameFor(reported string, model types.Model) string {
	if reported != "" {
		return reported
	}
	return model.ModelIdentifier
}

// -- forwarding -------------------------------------------------------------

func (c *upstreamClient) forwardAnthropic(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	baseURL, _ := p.Config["base_url"].(string)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	body, err := anthropicBody(req, model)
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "application/json, text/event-stream")
	if apiKey, _ := p.Config["api_key"].(string); apiKey != "" {
		upstreamReq.Header.Set("x-api-key", apiKey)
	}
	version, _ := p.Config["api_version"].(string)
	if version == "" {
		version = anthropicVersion
	}
	upstreamReq.Header.Set("anthropic-version", version)

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
		usage, err := relayAnthropicStream(flushWriter{w}, resp.Body, model, req.wantsUsageFrame())
		if err != nil {
			return outcome{}, err
		}
		return outcome{StatusSuccess: true, Usage: usage}, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{}, err
	}
	translated, usage, err := anthropicToOpenAI(raw, model)
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
