// Package bedrock implements provider.Provider on top of AWS Bedrock
// Runtime. The adapter uses the unified Converse API so a single code path
// works across Anthropic Claude, Meta Llama, Mistral, Cohere, Amazon Titan,
// and any future model family surfaced by Bedrock.
//
// Fluxa signs requests with its own in-tree SigV4 signer instead of pulling
// in the AWS SDK. This keeps the single-binary promise: the distroless
// image still weighs under 15 MiB even with Bedrock support enabled.
package bedrock

import (
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

// Adapter talks to Bedrock Runtime in one region.
type Adapter struct {
	name         string
	region       string
	accessKey    string
	secretKey    string
	sessionToken string
	baseURL      string
	models       []string
	client       *http.Client
}

// Options configures a new Bedrock Adapter.
type Options struct {
	Name         string
	Region       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	Models       []string
	Timeout      time.Duration
	// BaseURL overrides the regional endpoint; useful for mocking in tests.
	BaseURL string
}

// New builds a Bedrock Adapter. The caller must supply AccessKey, SecretKey
// and Region. SessionToken is optional and is forwarded as-is when set.
func New(opts Options) (*Adapter, error) {
	if opts.Region == "" {
		return nil, errors.New("bedrock: region is required")
	}
	if opts.AccessKey == "" || opts.SecretKey == "" {
		return nil, errors.New("bedrock: access_key and secret_key are required")
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", opts.Region)
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Adapter{
		name:         opts.Name,
		region:       opts.Region,
		accessKey:    opts.AccessKey,
		secretKey:    opts.SecretKey,
		sessionToken: opts.SessionToken,
		baseURL:      base,
		models:       opts.Models,
		client:       &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the configured provider name.
func (a *Adapter) Name() string { return a.name }

// Models returns the static model list.
func (a *Adapter) Models() []string {
	out := make([]string, len(a.models))
	copy(out, a.models)
	return out
}

// Chat performs a non-streaming /model/{id}/converse call and translates
// the response to OpenAI shape.
func (a *Adapter) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	converseReq, err := translateRequest(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(converseReq)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, "converse", body, "application/json")
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

	var converseResp converseResponse
	if err := json.Unmarshal(raw, &converseResp); err != nil {
		return nil, fmt.Errorf("bedrock: decode response: %w", err)
	}
	openaiResp, usage := translateResponse(req.Model, &converseResp)
	encoded, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, err
	}
	return &provider.ChatResponse{Model: req.Model, Usage: usage, Raw: encoded}, nil
}

// ChatStream opens a /model/{id}/converse-stream call and relays each
// EventStream frame as an OpenAI-shaped chat.completion.chunk.
func (a *Adapter) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	converseReq, err := translateRequest(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(converseReq)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, "converse-stream", body, "application/json")
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
		err := readEventStream(resp.Body, func(frame eventFrame) error {
			if frame.MessageType == "exception" {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- provider.StreamEvent{Err: fmt.Errorf("bedrock: %s: %s", frame.EventType, frame.Payload)}:
					return errors.New("stopped")
				}
			}
			chunk, ok := translateStreamFrame(req.Model, frame)
			if !ok {
				return nil
			}
			encoded, err := json.Marshal(chunk)
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- provider.StreamEvent{Data: encoded}:
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) && err.Error() != "stopped" {
			select {
			case out <- provider.StreamEvent{Err: err}:
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- provider.StreamEvent{Data: []byte("[DONE]")}:
		case <-ctx.Done():
		}
	}()
	return out, nil
}

// Health signs an empty GET against the region endpoint to verify the
// credentials. Bedrock does not expose a cheap "ping" endpoint, so the
// cheapest safe probe is a HEAD against /.
func (a *Adapter) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/", nil)
	if err != nil {
		return err
	}
	a.sign(req, nil)
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

// do builds, signs, and dispatches a POST to /model/{id}/{action}.
func (a *Adapter) do(ctx context.Context, model, action string, body []byte, contentType string) (*http.Response, error) {
	u := fmt.Sprintf("%s/model/%s/%s", a.baseURL, url.PathEscape(model), action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	a.sign(req, body)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "upstream request failed", Cause: err}
	}
	return resp, nil
}

// sign applies SigV4 authentication to req.
func (a *Adapter) sign(req *http.Request, body []byte) {
	signV4(req, body, a.accessKey, a.secretKey, a.sessionToken, a.region, "bedrock", time.Now())
}

// -- wire types ----------------------------------------------------------

type converseRequest struct {
	Messages        []converseMessage `json:"messages"`
	System          []converseText    `json:"system,omitempty"`
	InferenceConfig *inferenceConfig  `json:"inferenceConfig,omitempty"`
}

type converseMessage struct {
	Role    string         `json:"role"`
	Content []converseText `json:"content"`
}

type converseText struct {
	Text string `json:"text"`
}

type inferenceConfig struct {
	MaxTokens     *int     `json:"maxTokens,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	TopP          *float64 `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

type converseResponse struct {
	Output struct {
		Message struct {
			Role    string         `json:"role"`
			Content []converseText `json:"content"`
		} `json:"message"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
		TotalTokens  int `json:"totalTokens"`
	} `json:"usage"`
	StopReason string `json:"stopReason"`
}

// -- translation helpers -------------------------------------------------

func translateRequest(req *provider.ChatRequest) (*converseRequest, error) {
	// When callers passed a raw OpenAI body, decode the fields we need.
	if len(req.Raw) > 0 && len(req.Messages) == 0 {
		if err := hydrateFromRaw(req); err != nil {
			return nil, err
		}
	}
	out := &converseRequest{}
	for _, m := range req.Messages {
		text, err := extractText(m.Content)
		if err != nil {
			return nil, err
		}
		switch m.Role {
		case provider.RoleSystem:
			out.System = append(out.System, converseText{Text: text})
		case provider.RoleAssistant:
			out.Messages = append(out.Messages, converseMessage{Role: "assistant", Content: []converseText{{Text: text}}})
		default:
			out.Messages = append(out.Messages, converseMessage{Role: "user", Content: []converseText{{Text: text}}})
		}
	}
	if req.Temperature != nil || req.TopP != nil || req.MaxTokens != nil || len(req.Stop) > 0 {
		out.InferenceConfig = &inferenceConfig{
			MaxTokens:     req.MaxTokens,
			Temperature:   req.Temperature,
			TopP:          req.TopP,
			StopSequences: req.Stop,
		}
	}
	return out, nil
}

func hydrateFromRaw(req *provider.ChatRequest) error {
	var payload struct {
		Messages    []provider.Message `json:"messages"`
		Temperature *float64           `json:"temperature"`
		TopP        *float64           `json:"top_p"`
		MaxTokens   *int               `json:"max_tokens"`
		Stop        []string           `json:"stop"`
	}
	if err := json.Unmarshal(req.Raw, &payload); err != nil {
		return fmt.Errorf("bedrock: decode raw body: %w", err)
	}
	req.Messages = payload.Messages
	req.Temperature = payload.Temperature
	req.TopP = payload.TopP
	req.MaxTokens = payload.MaxTokens
	req.Stop = payload.Stop
	return nil
}

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
		return "", fmt.Errorf("bedrock: unsupported content shape")
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" || p.Type == "" {
			b.WriteString(p.Text)
		}
	}
	return b.String(), nil
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   provider.Usage `json:"usage"`
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

func translateResponse(model string, r *converseResponse) (*openAIResponse, provider.Usage) {
	var b strings.Builder
	for _, p := range r.Output.Message.Content {
		b.WriteString(p.Text)
	}
	usage := provider.Usage{
		PromptTokens:     r.Usage.InputTokens,
		CompletionTokens: r.Usage.OutputTokens,
		TotalTokens:      r.Usage.TotalTokens,
	}
	return &openAIResponse{
		ID:     "bedrock-resp",
		Object: "chat.completion",
		Model:  model,
		Choices: []openAIChoice{{
			Index:        0,
			Message:      openAIMessage{Role: "assistant", Content: b.String()},
			FinishReason: mapStopReason(r.StopReason),
		}},
		Usage: usage,
	}, usage
}

// openAIChunk matches a single OpenAI streaming chat.completion.chunk.
type openAIChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Model   string             `json:"model"`
	Choices []openAIChunkDelta `json:"choices"`
}

type openAIChunkDelta struct {
	Index        int              `json:"index"`
	Delta        openAIChunkInner `json:"delta"`
	FinishReason string           `json:"finish_reason,omitempty"`
}

type openAIChunkInner struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// translateStreamFrame maps a Bedrock EventStream frame to an OpenAI chunk.
// Frames we do not care about (messageStart, contentBlockStart with no
// text, metadata) are translated to empty chunks which the caller drops.
func translateStreamFrame(model string, frame eventFrame) (*openAIChunk, bool) {
	switch frame.EventType {
	case "contentBlockDelta":
		var payload struct {
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil || payload.Delta.Text == "" {
			return nil, false
		}
		return &openAIChunk{
			ID:     "bedrock-stream",
			Object: "chat.completion.chunk",
			Model:  model,
			Choices: []openAIChunkDelta{{
				Index: 0,
				Delta: openAIChunkInner{Role: "assistant", Content: payload.Delta.Text},
			}},
		}, true
	case "messageStop":
		var payload struct {
			StopReason string `json:"stopReason"`
		}
		_ = json.Unmarshal(frame.Payload, &payload)
		return &openAIChunk{
			ID:     "bedrock-stream",
			Object: "chat.completion.chunk",
			Model:  model,
			Choices: []openAIChunkDelta{{
				Index:        0,
				FinishReason: mapStopReason(payload.StopReason),
			}},
		}, true
	default:
		return nil, false
	}
}

// mapStopReason maps Bedrock stop reasons to OpenAI-style strings.
func mapStopReason(r string) string {
	switch r {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "content_filtered":
		return "content_filter"
	default:
		if r == "" {
			return ""
		}
		return strings.ToLower(r)
	}
}

// upstreamError normalises a Bedrock error JSON envelope.
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Message string `json:"message"`
		Type    string `json:"__type"`
	}
	_ = json.Unmarshal(body, &envelope)
	msg := envelope.Message
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &provider.Error{Status: status, Code: envelope.Type, Message: msg}
}
