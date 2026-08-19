package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The Anthropic Messages request shape, coming in.
//
// /v1/messages is what Claude Code and every Anthropic SDK speak, and
// having it is what lets those clients point at Fluxa at all. The
// translation runs inbound rather than the endpoint being a proxy so
// that one Anthropic-speaking client reaches *any* configured provider,
// not only an Anthropic one -- a deployment that bought OpenAI capacity
// can still put Claude Code in front of it.
//
// This is the mirror image of upstream_anthropic.go: the same disagreements
// between the two protocols, read in the other direction.

type messagesRequest struct {
	Model         string              `json:"model"`
	MaxTokens     int                 `json:"max_tokens"`
	Messages      []messagesInMessage `json:"messages"`
	System        json.RawMessage     `json:"system"`
	Temperature   *float64            `json:"temperature"`
	TopP          *float64            `json:"top_p"`
	StopSequences []string            `json:"stop_sequences"`
	Stream        bool                `json:"stream"`
	Tools         []messagesInTool    `json:"tools"`
	ToolChoice    json.RawMessage     `json:"tool_choice"`
}

type messagesInMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type messagesInTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func decodeMessagesRequest(r io.Reader) (messagesRequest, error) {
	var req messagesRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return messagesRequest{}, err
	}
	if req.MaxTokens <= 0 {
		// Required by the Anthropic API, so a caller reaching here
		// without it is sending something Anthropic would reject too.
		return messagesRequest{}, fmt.Errorf("gateway: messages: max_tokens is required")
	}
	return req, nil
}

// toChatRequest turns an Anthropic Messages request into the canonical
// OpenAI-shaped request the rest of the pipeline works in.
func (req messagesRequest) toChatRequest() (chatRequest, error) {
	var messages []chatMessage

	if system, err := messagesSystemText(req.System); err != nil {
		return chatRequest{}, err
	} else if system != "" {
		messages = append(messages, newChatMessage("system", system, nil))
	}

	for _, m := range req.Messages {
		converted, err := messagesTurnToChat(m)
		if err != nil {
			return chatRequest{}, err
		}
		messages = append(messages, converted...)
	}

	raw := map[string]json.RawMessage{}
	put := func(key string, v any) error {
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		raw[key] = encoded
		return nil
	}

	if err := put("model", req.Model); err != nil {
		return chatRequest{}, err
	}
	if err := put("max_tokens", req.MaxTokens); err != nil {
		return chatRequest{}, err
	}
	if req.Stream {
		raw["stream"] = json.RawMessage("true")
	}
	if req.Temperature != nil {
		if err := put("temperature", req.Temperature); err != nil {
			return chatRequest{}, err
		}
	}
	if req.TopP != nil {
		if err := put("top_p", req.TopP); err != nil {
			return chatRequest{}, err
		}
	}
	if len(req.StopSequences) > 0 {
		if err := put("stop", req.StopSequences); err != nil {
			return chatRequest{}, err
		}
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.InputSchema
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  schema,
				},
			})
		}
		if err := put("tools", tools); err != nil {
			return chatRequest{}, err
		}
	}
	if choice, ok, err := messagesToolChoice(req.ToolChoice); err != nil {
		return chatRequest{}, err
	} else if ok {
		raw["tool_choice"] = choice
	}

	// The messages are held on the struct rather than in raw: the DLP
	// pass mutates them, and body() re-serializes from the struct.
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return chatRequest{}, err
	}
	raw["messages"] = messagesJSON

	return chatRequest{
		raw:       raw,
		Model:     req.Model,
		Messages:  messages,
		Stream:    req.Stream,
		MaxTokens: req.MaxTokens,
	}, nil
}

// messagesSystemText reads Anthropic's system field, which is a string
// or a list of text blocks.
func messagesSystemText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	if s, err := jsonString(raw); err == nil {
		return s, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("gateway: messages: system: %w", err)
	}
	var parts []string
	for _, b := range blocks {
		if b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// messagesTurnToChat converts one Anthropic turn. It can produce more
// than one message: a user turn carrying tool results becomes one "tool"
// message per result, because OpenAI models each result as its own turn
// where Anthropic packs them into one.
func messagesTurnToChat(m messagesInMessage) ([]chatMessage, error) {
	if s, err := jsonString(m.Content); err == nil {
		return []chatMessage{newChatMessage(m.Role, s, nil)}, nil
	}

	var blocks []anthropicContent
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil, fmt.Errorf("gateway: messages: content: %w", err)
	}

	var text strings.Builder
	var toolCalls []openAIToolCall
	var out []chatMessage

	for _, block := range blocks {
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
		case "tool_result":
			out = append(out, newChatMessage("tool", block.Content, map[string]any{
				"tool_call_id": block.ToolUseID,
			}))
		}
	}

	if text.Len() > 0 || len(toolCalls) > 0 {
		extra := map[string]any{}
		if len(toolCalls) > 0 {
			extra["tool_calls"] = toolCalls
		}
		turn := newChatMessage(m.Role, text.String(), extra)
		// An assistant turn that is only tool calls carries a null
		// content in OpenAI's shape, not an empty string.
		if text.Len() == 0 && len(toolCalls) > 0 {
			turn.hasTextContent = false
			delete(turn.raw, "content")
		}
		out = append([]chatMessage{turn}, out...)
	}

	return out, nil
}

// messagesToolChoice maps Anthropic's tool_choice onto OpenAI's.
func messagesToolChoice(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}
	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, false, fmt.Errorf("gateway: messages: tool_choice: %w", err)
	}
	switch choice.Type {
	case "auto":
		return json.RawMessage(`"auto"`), true, nil
	case "any":
		return json.RawMessage(`"required"`), true, nil
	case "none":
		return json.RawMessage(`"none"`), true, nil
	case "tool":
		if choice.Name == "" {
			return nil, false, nil
		}
		return json.RawMessage(fmt.Sprintf(`{"type":"function","function":{"name":%q}}`, choice.Name)), true, nil
	}
	return nil, false, nil
}

// newChatMessage builds a canonical message with the raw object body()
// re-serializes from, so a message synthesised here behaves exactly like
// one decoded off an OpenAI request.
func newChatMessage(role, content string, extra map[string]any) chatMessage {
	raw := map[string]json.RawMessage{}
	if encoded, err := json.Marshal(role); err == nil {
		raw["role"] = encoded
	}
	if encoded, err := json.Marshal(content); err == nil {
		raw["content"] = encoded
	}
	for k, v := range extra {
		if encoded, err := json.Marshal(v); err == nil {
			raw[k] = encoded
		}
	}
	return chatMessage{raw: raw, Role: role, Content: content, hasTextContent: true}
}
