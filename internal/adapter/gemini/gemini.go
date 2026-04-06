// Package gemini implements provider.Provider on top of the Google Gemini
// (Generative Language) REST API. Unlike the Chinese vendors, Gemini does
// not ship a fully OpenAI-compatible surface: messages are represented as
// "contents" with parts, system prompts live in a dedicated
// systemInstruction field, and the streaming SSE payload is a sequence of
// GenerateContentResponse JSON objects. This adapter translates between the
// OpenAI wire format Fluxa standardises on and the Gemini shape in both
// directions so callers never have to care.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/provider"
)

// Adapter speaks the native Gemini REST API.
type Adapter struct {
	name    string
	baseURL string
	apiKey  string
	models  []string
	headers map[string]string
	client  *http.Client
}

// Options configures a new Gemini Adapter.
type Options struct {
	Name    string
	BaseURL string // defaults to https://generativelanguage.googleapis.com/v1beta
	APIKey  string
	Models  []string
	Headers map[string]string
	Timeout time.Duration
}

// New builds a Gemini Adapter.
func New(opts Options) *Adapter {
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Adapter{
		name:    opts.Name,
		baseURL: base,
		apiKey:  opts.APIKey,
		models:  opts.Models,
		headers: opts.Headers,
		client:  &http.Client{Timeout: timeout},
	}
}

// Name returns the configured provider name.
func (a *Adapter) Name() string { return a.name }

// Models returns the configured static model list.
func (a *Adapter) Models() []string {
	out := make([]string, len(a.models))
	copy(out, a.models)
	return out
}

// Chat performs a non-streaming :generateContent call and translates the
// response back to an OpenAI-shaped envelope.
func (a *Adapter) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	geminiReq, err := translateRequest(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, "generateContent", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "read upstream body", Cause: err}
	}
	if resp.StatusCode >= 400 {
		return nil, upstreamError(resp.StatusCode, raw)
	}

	var gResp generateContentResponse
	if err := json.Unmarshal(raw, &gResp); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}
	openaiResp, usage := translateResponse(req.Model, &gResp)
	encoded, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, err
	}
	return &provider.ChatResponse{Model: req.Model, Usage: usage, Raw: encoded}, nil
}

// ChatStream opens an SSE connection to :streamGenerateContent and emits
// OpenAI-shaped chunks on the returned channel.
func (a *Adapter) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	geminiReq, err := translateRequest(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, "streamGenerateContent?alt=sse", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, upstreamError(resp.StatusCode, raw)
	}

	out := make(chan provider.StreamEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 {
				continue
			}
			var g generateContentResponse
			if err := json.Unmarshal(payload, &g); err != nil {
				continue
			}
			chunk := translateStreamChunk(req.Model, &g)
			encoded, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- provider.StreamEvent{Data: encoded}:
			}
		}
		// Emit the OpenAI-style [DONE] sentinel so downstream handlers can
		// treat every stream path identically.
		select {
		case out <- provider.StreamEvent{Data: []byte("[DONE]")}:
		case <-ctx.Done():
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			select {
			case out <- provider.StreamEvent{Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

// Health performs a cheap GET /models check.
func (a *Adapter) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u := a.baseURL + "/models?key=" + url.QueryEscape(a.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	return nil
}

// do dispatches a POST to /models/{model}:{action}. The API key travels as
// an x-goog-api-key header so it never shows up in access logs.
func (a *Adapter) do(ctx context.Context, model, action string, body []byte) (*http.Response, error) {
	u := fmt.Sprintf("%s/models/%s:%s", a.baseURL, url.PathEscape(model), action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", a.apiKey)
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "upstream request failed", Cause: err}
	}
	return resp, nil
}

// -- wire types ----------------------------------------------------------

type generateContentRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationCfg   `json:"generationConfig,omitempty"`
	SafetySettings    []map[string]any `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type generationCfg struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// -- translation helpers -------------------------------------------------

// translateRequest converts an OpenAI ChatRequest into the Gemini request
// shape. It handles system messages by lifting them into
// systemInstruction, supports string and multimodal text content, and
// forwards sampling parameters into generationConfig.
func translateRequest(req *provider.ChatRequest) (*generateContentRequest, error) {
	// When the caller sent a raw OpenAI body, decode it to extract the
	// fields we need. This preserves parameters such as max_tokens and
	// temperature that live outside our typed struct.
	if len(req.Raw) > 0 && len(req.Messages) == 0 {
		if err := hydrateFromRaw(req); err != nil {
			return nil, err
		}
	}

	out := &generateContentRequest{}
	for _, m := range req.Messages {
		text, err := extractText(m.Content)
		if err != nil {
			return nil, err
		}
		switch m.Role {
		case provider.RoleSystem:
			out.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: text}}}
		case provider.RoleAssistant:
			out.Contents = append(out.Contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: text}}})
		default:
			out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: text}}})
		}
	}

	if req.Temperature != nil || req.TopP != nil || req.MaxTokens != nil || len(req.Stop) > 0 {
		out.GenerationConfig = &generationCfg{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		}
	}
	return out, nil
}

// hydrateFromRaw decodes the portion of a raw OpenAI body we need so that
// translateRequest can populate the Gemini payload. Fields that Gemini does
// not understand are silently dropped.
func hydrateFromRaw(req *provider.ChatRequest) error {
	var payload struct {
		Messages    []provider.Message `json:"messages"`
		Temperature *float64           `json:"temperature"`
		TopP        *float64           `json:"top_p"`
		MaxTokens   *int               `json:"max_tokens"`
		Stop        []string           `json:"stop"`
	}
	if err := json.Unmarshal(req.Raw, &payload); err != nil {
		return fmt.Errorf("gemini: decode raw body: %w", err)
	}
	req.Messages = payload.Messages
	req.Temperature = payload.Temperature
	req.TopP = payload.TopP
	req.MaxTokens = payload.MaxTokens
	req.Stop = payload.Stop
	return nil
}

// extractText pulls a plain text payload out of an OpenAI content field,
// which can be either a JSON string or an array of {"type":"text","text":"..."}
// parts. Image and audio parts are ignored for v4.0; native multimodal
// forwarding is tracked for v5.0.
func extractText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("gemini: unsupported content shape")
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" || p.Type == "" {
			b.WriteString(p.Text)
		}
	}
	return b.String(), nil
}

// openAIResponse mirrors the subset of OpenAI fields we fill in after
// translating a Gemini response.
type openAIResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Model   string                `json:"model"`
	Choices []openAIChoice        `json:"choices"`
	Usage   provider.Usage        `json:"usage"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func translateResponse(model string, g *generateContentResponse) (*openAIResponse, provider.Usage) {
	resp := &openAIResponse{
		ID:     "gemini-resp",
		Object: "chat.completion",
		Model:  model,
	}
	for i, cand := range g.Candidates {
		var b strings.Builder
		for _, p := range cand.Content.Parts {
			b.WriteString(p.Text)
		}
		resp.Choices = append(resp.Choices, openAIChoice{
			Index:        i,
			Message:      openAIMessage{Role: "assistant", Content: b.String()},
			FinishReason: mapFinishReason(cand.FinishReason),
		})
	}
	usage := provider.Usage{
		PromptTokens:     g.UsageMetadata.PromptTokenCount,
		CompletionTokens: g.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      g.UsageMetadata.TotalTokenCount,
	}
	resp.Usage = usage
	return resp, usage
}

// openAIChunk matches a single OpenAI streaming chat.completion.chunk event.
type openAIChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Model   string             `json:"model"`
	Choices []openAIChunkDelta `json:"choices"`
}

type openAIChunkDelta struct {
	Index        int               `json:"index"`
	Delta        openAIChunkInner  `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

type openAIChunkInner struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func translateStreamChunk(model string, g *generateContentResponse) *openAIChunk {
	chunk := &openAIChunk{
		ID:     "gemini-stream",
		Object: "chat.completion.chunk",
		Model:  model,
	}
	for i, cand := range g.Candidates {
		var b strings.Builder
		for _, p := range cand.Content.Parts {
			b.WriteString(p.Text)
		}
		chunk.Choices = append(chunk.Choices, openAIChunkDelta{
			Index:        i,
			Delta:        openAIChunkInner{Role: "assistant", Content: b.String()},
			FinishReason: mapFinishReason(cand.FinishReason),
		})
	}
	return chunk
}

// mapFinishReason converts Gemini's finish reason enum to the closest
// OpenAI equivalent so downstream tooling can rely on familiar strings.
func mapFinishReason(g string) string {
	switch g {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		if g == "" {
			return ""
		}
		return strings.ToLower(g)
	}
}

// upstreamError normalises a Gemini error body into a provider.Error.
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	msg := envelope.Error.Message
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &provider.Error{
		Status:  status,
		Code:    envelope.Error.Status,
		Message: msg,
	}
}
