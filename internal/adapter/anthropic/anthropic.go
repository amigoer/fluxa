// Package anthropic implements the provider.MessagesProvider interface on top
// of the Anthropic /v1/messages API. The adapter is a byte-for-byte
// passthrough: it never translates Claude's native request or response shape
// so that clients which depend on Anthropic-specific fields (thinking blocks,
// tool_use, cache_control) receive them exactly as sent by the upstream.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/provider"
)

// Adapter speaks the Anthropic Messages API.
type Adapter struct {
	name       string
	baseURL    string
	apiKey     string
	apiVersion string
	models     []string
	headers    map[string]string
	client     *http.Client
}

// Options configures a new Anthropic Adapter.
type Options struct {
	Name       string
	BaseURL    string
	APIKey     string
	APIVersion string
	Models     []string
	Headers    map[string]string
	Timeout    time.Duration
}

// New builds an Anthropic Adapter. BaseURL defaults to the public Anthropic
// endpoint and APIVersion defaults to the pinned 2023-06-01 version required
// by the Messages API.
func New(opts Options) *Adapter {
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	version := opts.APIVersion
	if version == "" {
		version = "2023-06-01"
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Adapter{
		name:       opts.Name,
		baseURL:    base,
		apiKey:     opts.APIKey,
		apiVersion: version,
		models:     opts.Models,
		headers:    opts.Headers,
		client:     &http.Client{Timeout: timeout},
	}
}

// Name returns the configured provider name.
func (a *Adapter) Name() string { return a.name }

// Models returns the static model list from configuration.
func (a *Adapter) Models() []string {
	out := make([]string, len(a.models))
	copy(out, a.models)
	return out
}

// Chat satisfies the Provider interface for v1.0 by returning an error: the
// OpenAI path is not yet translated to the Anthropic wire format. Clients
// targetting Claude models should hit /v1/messages instead.
//
// Cross-protocol translation (OpenAI shape → Anthropic shape) is planned for
// v2.0 so that existing OpenAI SDKs can call Claude without any changes.
func (a *Adapter) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, &provider.Error{
		Status:  http.StatusNotImplemented,
		Message: "anthropic adapter does not yet support /v1/chat/completions; call /v1/messages instead",
	}
}

// ChatStream mirrors Chat for the streaming path.
func (a *Adapter) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, &provider.Error{
		Status:  http.StatusNotImplemented,
		Message: "anthropic adapter does not yet support /v1/chat/completions streaming; call /v1/messages instead",
	}
}

// Messages issues a non-streaming /v1/messages call and returns the upstream
// body verbatim.
func (a *Adapter) Messages(ctx context.Context, req *provider.MessagesRequest) (*provider.MessagesResponse, error) {
	body, err := setStreamFlag(req.Raw, false)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, body)
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

	var parsed struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(raw, &parsed)

	return &provider.MessagesResponse{
		Model: parsed.Model,
		Usage: provider.Usage{
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		},
		Raw: raw,
	}, nil
}

// MessagesStream opens an SSE connection to /v1/messages?stream=true and
// relays every chunk to the returned channel. Anthropic's SSE format emits
// both "event:" and "data:" lines; this adapter forwards only the "data:"
// payloads which carry the JSON event envelope expected by Anthropic SDKs.
func (a *Adapter) MessagesStream(ctx context.Context, req *provider.MessagesRequest) (<-chan provider.StreamEvent, error) {
	body, err := setStreamFlag(req.Raw, true)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, body)
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
		var currentEvent string
		for scanner.Scan() {
			line := scanner.Bytes()
			switch {
			case len(line) == 0:
				currentEvent = ""
			case bytes.HasPrefix(line, []byte("event:")):
				currentEvent = strings.TrimSpace(string(line[len("event:"):]))
			case bytes.HasPrefix(line, []byte("data:")):
				payload := bytes.TrimSpace(line[len("data:"):])
				if len(payload) == 0 {
					continue
				}
				// Prefix the event name so downstream handlers can emit a
				// valid "event: X\ndata: Y\n\n" SSE frame to the client.
				chunk := make([]byte, 0, len(currentEvent)+1+len(payload))
				chunk = append(chunk, currentEvent...)
				chunk = append(chunk, '|')
				chunk = append(chunk, payload...)
				select {
				case <-ctx.Done():
					return
				case out <- provider.StreamEvent{Data: chunk}:
				}
				if currentEvent == "message_stop" {
					return
				}
			}
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

// Health performs a cheap HEAD-style call by issuing a tiny dummy Messages
// request. Anthropic does not offer a free health endpoint, so we validate
// credentials with the smallest possible payload and ignore 4xx bodies.
func (a *Adapter) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	a.applyHeaders(req)
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

// do issues a POST /v1/messages request with the supplied body.
func (a *Adapter) do(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	a.applyHeaders(req)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "upstream request failed", Cause: err}
	}
	return resp, nil
}

// applyHeaders sets the Anthropic-specific authentication and versioning
// headers on every outbound request.
func (a *Adapter) applyHeaders(r *http.Request) {
	if a.apiKey != "" {
		r.Header.Set("x-api-key", a.apiKey)
	}
	if a.apiVersion != "" {
		r.Header.Set("anthropic-version", a.apiVersion)
	}
	for k, v := range a.headers {
		r.Header.Set(k, v)
	}
}

// setStreamFlag rewrites the top-level "stream" flag on a JSON object.
func setStreamFlag(src []byte, stream bool) ([]byte, error) {
	if len(src) == 0 {
		return nil, errors.New("empty request body")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return nil, fmt.Errorf("decode messages body: %w", err)
	}
	if stream {
		obj["stream"] = json.RawMessage(`true`)
	} else {
		delete(obj, "stream")
	}
	return json.Marshal(obj)
}

// upstreamError normalises an Anthropic error body.
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
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
		Code:    envelope.Error.Type,
		Message: msg,
	}
}
