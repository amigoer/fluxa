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

// Google's Gemini generateContent API, translated both ways.
//
// The translation removed in f5b192a handled text only: it pulled a
// string out of each message and dropped everything else, so tools never
// reached the model and a caller using function calling watched it
// silently never call anything. That is the same failure mode as
// re-marshalling a narrow struct, and it is not reproduced here.
//
// Where Gemini genuinely differs from OpenAI:
//
//   - the assistant role is called "model"
//   - the system prompt is systemInstruction, outside the turn list
//   - a turn is a list of parts, and a tool call or its result is a part
//     rather than a field beside the content
//   - a functionResponse names the function it answers instead of
//     carrying the call's id, so the id has to be resolved back to a
//     name from earlier in the conversation
//   - sampling parameters live under generationConfig

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

type geminiRequest struct {
	Contents          []geminiContent   `json:"contents"`
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig  `json:"generationConfig,omitempty"`
	Tools             []geminiToolGroup `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig `json:"toolConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiToolGroup struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig struct {
		Mode                 string   `json:"mode"`
		AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
	} `json:"functionCallingConfig"`
}

// geminiBody translates an OpenAI chat-completions request.
func geminiBody(req chatRequest) ([]byte, error) {
	out := geminiRequest{}

	// Gemini matches a tool result to its call by function name, while
	// OpenAI matches by call id. Walk the assistant turns first so a
	// later "tool" message can resolve its id back to the name.
	callNames := map[string]string{}
	for _, m := range req.Messages {
		if m.Role != "assistant" {
			continue
		}
		raw, ok := m.raw["tool_calls"]
		if !ok || string(raw) == "null" {
			continue
		}
		var calls []openAIToolCall
		if err := json.Unmarshal(raw, &calls); err != nil {
			return nil, fmt.Errorf("gateway: gemini: tool_calls: %w", err)
		}
		for _, call := range calls {
			callNames[call.ID] = call.Function.Name
		}
	}

	var systemParts []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			if m.Content != "" {
				systemParts = append(systemParts, m.Content)
			}

		case "tool":
			id, _ := jsonString(m.raw["tool_call_id"])
			name := callNames[id]
			if name == "" {
				name = id
			}
			out.Contents = append(out.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{FunctionResponse: &geminiFunctionResponse{
					Name:     name,
					Response: geminiToolResponse(m.Content),
				}}},
			})

		case "assistant":
			parts, err := geminiAssistantParts(m)
			if err != nil {
				return nil, err
			}
			if len(parts) == 0 {
				continue
			}
			out.Contents = append(out.Contents, geminiContent{Role: "model", Parts: parts})

		default:
			out.Contents = append(out.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	if len(systemParts) > 0 {
		out.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: strings.Join(systemParts, "\n\n")}}}
	}

	cfg := geminiGenConfig{MaxOutputTokens: req.MaxTokens}
	if err := decodeField(req.raw, "temperature", &cfg.Temperature); err != nil {
		return nil, err
	}
	if err := decodeField(req.raw, "top_p", &cfg.TopP); err != nil {
		return nil, err
	}
	if err := decodeStopSequences(req.raw, &cfg.StopSequences); err != nil {
		return nil, err
	}
	if cfg.Temperature != nil || cfg.TopP != nil || cfg.MaxOutputTokens > 0 || len(cfg.StopSequences) > 0 {
		out.GenerationConfig = &cfg
	}

	if err := geminiTools(req.raw, &out); err != nil {
		return nil, err
	}

	return json.Marshal(out)
}

// geminiToolResponse wraps a tool's output. Gemini wants an object
// there; a tool that returned bare text gets one built around it.
func geminiToolResponse(content string) json.RawMessage {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") && json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	wrapped, err := json.Marshal(map[string]string{"result": content})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return wrapped
}

func geminiAssistantParts(m chatMessage) ([]geminiPart, error) {
	var parts []geminiPart
	if m.Content != "" {
		parts = append(parts, geminiPart{Text: m.Content})
	}

	raw, ok := m.raw["tool_calls"]
	if !ok || string(raw) == "null" {
		return parts, nil
	}
	var calls []openAIToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return nil, fmt.Errorf("gateway: gemini: tool_calls: %w", err)
	}
	for _, call := range calls {
		args := call.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
			Name: call.Function.Name,
			Args: json.RawMessage(args),
		}})
	}
	return parts, nil
}

func geminiTools(raw map[string]json.RawMessage, out *geminiRequest) error {
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
			return fmt.Errorf("gateway: gemini: tools: %w", err)
		}
		group := geminiToolGroup{}
		for _, t := range tools {
			group.FunctionDeclarations = append(group.FunctionDeclarations, geminiFunctionDecl{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}
		if len(group.FunctionDeclarations) > 0 {
			out.Tools = []geminiToolGroup{group}
		}
	}

	choiceRaw, ok := raw["tool_choice"]
	if !ok || string(choiceRaw) == "null" {
		return nil
	}
	cfg := &geminiToolConfig{}
	if s, err := jsonString(choiceRaw); err == nil {
		switch s {
		case "auto":
			cfg.FunctionCallingConfig.Mode = "AUTO"
		case "required":
			cfg.FunctionCallingConfig.Mode = "ANY"
		case "none":
			cfg.FunctionCallingConfig.Mode = "NONE"
		default:
			return nil
		}
		out.ToolConfig = cfg
		return nil
	}
	var named struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(choiceRaw, &named); err != nil {
		return fmt.Errorf("gateway: gemini: tool_choice: %w", err)
	}
	if named.Function.Name != "" {
		cfg.FunctionCallingConfig.Mode = "ANY"
		cfg.FunctionCallingConfig.AllowedFunctionNames = []string{named.Function.Name}
		out.ToolConfig = cfg
	}
	return nil
}

// -- response translation ---------------------------------------------------

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

func mapGeminiFinishReason(reason string, sawToolCall bool) string {
	if sawToolCall {
		return "tool_calls"
	}
	switch reason {
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT":
		return "content_filter"
	case "":
		return ""
	default:
		return "stop"
	}
}

// geminiParts pulls text and tool calls out of a candidate's parts.
// Gemini's functionCall carries no id, so one is synthesised -- the same
// id then comes back on the caller's next turn and is resolved to a name
// again by geminiBody.
func geminiParts(parts []geminiPart, idPrefix string) (string, []openAIToolCall) {
	var text strings.Builder
	var calls []openAIToolCall
	for _, part := range parts {
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall == nil {
			continue
		}
		call := openAIToolCall{ID: fmt.Sprintf("%s_%d", idPrefix, len(calls)), Type: "function"}
		call.Function.Name = part.FunctionCall.Name
		call.Function.Arguments = string(part.FunctionCall.Args)
		if call.Function.Arguments == "" {
			call.Function.Arguments = "{}"
		}
		calls = append(calls, call)
	}
	return text.String(), calls
}

func geminiToOpenAI(raw []byte, model types.Model) ([]byte, *chatUsage, error) {
	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("gateway: gemini: decode response: %w", err)
	}

	var text string
	var calls []openAIToolCall
	finishReason := ""
	if len(parsed.Candidates) > 0 {
		text, calls = geminiParts(parsed.Candidates[0].Content.Parts, "call")
		finishReason = mapGeminiFinishReason(parsed.Candidates[0].FinishReason, len(calls) > 0)
	}

	usage := &chatUsage{
		PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
		CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
	}

	message := map[string]any{"role": "assistant", "content": text}
	if len(calls) > 0 {
		message["tool_calls"] = calls
		message["content"] = nil
	}

	body, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model.ModelIdentifier,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
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

func (c *upstreamClient) forwardGemini(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	baseURL, _ := p.Config["base_url"].(string)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	apiVersion, _ := p.Config["api_version"].(string)
	if apiVersion == "" {
		apiVersion = "v1beta"
	}

	action := "generateContent"
	query := ""
	if req.Stream {
		action = "streamGenerateContent"
		query = "?alt=sse"
	}
	endpoint := fmt.Sprintf("%s/%s/models/%s:%s%s",
		baseURL, apiVersion, url.PathEscape(model.ModelIdentifier), action, query)

	body, err := geminiBody(req)
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	// The key goes in a header rather than the query string: a URL with
	// a credential in it ends up in proxy and access logs.
	if apiKey, _ := p.Config["api_key"].(string); apiKey != "" {
		upstreamReq.Header.Set("x-goog-api-key", apiKey)
	}

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
		usage, err := relayGeminiStream(flushWriter{w}, resp.Body, model, req.wantsUsageFrame())
		if err != nil {
			return outcome{}, err
		}
		return outcome{StatusSuccess: true, Usage: usage}, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{}, err
	}
	translated, usage, err := geminiToOpenAI(raw, model)
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
