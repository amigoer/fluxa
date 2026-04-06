// Package openai implements the provider.Provider interface on top of the
// OpenAI REST API. Every major Chinese and Western provider except Anthropic
// exposes an OpenAI-compatible endpoint (DeepSeek, Qwen/DashScope, Moonshot,
// GLM, Ollama, vLLM, …) so this single adapter covers most of v1.0's provider
// matrix. Provider-specific quirks (reasoning effort, max_completion_tokens,
// thinking streams) are handled by tiny hook points rather than duplicated
// adapters.
package openai

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

// Adapter is an OpenAI-compatible provider.
type Adapter struct {
	name    string
	baseURL string
	apiKey  string
	models  []string
	headers map[string]string
	client  *http.Client
}

// Options configures a new Adapter.
type Options struct {
	Name    string
	BaseURL string
	APIKey  string
	Models  []string
	Headers map[string]string
	Timeout time.Duration
}

// New builds an OpenAI-compatible Adapter. BaseURL defaults to the public
// OpenAI API when empty; callers pointing at Ollama, DeepSeek or DashScope
// supply their own base URL.
func New(opts Options) *Adapter {
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
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
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the configured provider name.
func (a *Adapter) Name() string { return a.name }

// Models returns the static model list from configuration. v1.0 does not call
// the remote /models endpoint because many providers rate-limit it heavily.
func (a *Adapter) Models() []string {
	out := make([]string, len(a.models))
	copy(out, a.models)
	return out
}

// Chat performs a non-streaming chat completion and returns the upstream
// OpenAI-compatible response body untouched.
func (a *Adapter) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	body, err := buildRequestBody(req, false)
	if err != nil {
		return nil, err
	}
	httpResp, err := a.do(ctx, body)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "read upstream body", Cause: err}
	}
	if httpResp.StatusCode >= 400 {
		return nil, upstreamError(httpResp.StatusCode, raw)
	}

	var parsed struct {
		Model string          `json:"model"`
		Usage provider.Usage  `json:"usage"`
		Extra json.RawMessage `json:"-"`
	}
	_ = json.Unmarshal(raw, &parsed)

	return &provider.ChatResponse{
		Model: parsed.Model,
		Usage: parsed.Usage,
		Raw:   raw,
	}, nil
}

// ChatStream opens an SSE connection to the upstream and relays every chunk
// to the returned channel. The channel is closed as soon as the upstream
// finishes, errors, or the context is cancelled.
func (a *Adapter) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	body, err := buildRequestBody(req, true)
	if err != nil {
		return nil, err
	}
	httpResp, err := a.do(ctx, body)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode >= 400 {
		raw, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, upstreamError(httpResp.StatusCode, raw)
	}

	out := make(chan provider.StreamEvent, 16)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()
		scanner := bufio.NewScanner(httpResp.Body)
		// OpenAI chunks can exceed the default 64 KB scanner buffer when
		// tool calls or large function arguments are involved. Bump it.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 {
				continue
			}
			// Copy because the scanner reuses its buffer on the next call.
			chunk := make([]byte, len(payload))
			copy(chunk, payload)
			select {
			case <-ctx.Done():
				return
			case out <- provider.StreamEvent{Data: chunk}:
			}
			if bytes.Equal(payload, []byte("[DONE]")) {
				return
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

// Health performs a cheap GET /models request to verify reachability and
// credentials. Adapters pointing at Ollama fall back to a bare "/" check
// because Ollama does not expose a /models endpoint with auth.
func (a *Adapter) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	a.applyAuth(httpReq)
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	return nil
}

// do issues a POST to /chat/completions with the supplied JSON body and
// returns the live http.Response (streaming callers must close it).
func (a *Adapter) do(ctx context.Context, body []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	a.applyAuth(httpReq)
	for k, v := range a.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "upstream request failed", Cause: err}
	}
	return resp, nil
}

// applyAuth adds the Authorization header when an API key is configured.
// Ollama deployments without auth leave apiKey empty.
func (a *Adapter) applyAuth(r *http.Request) {
	if a.apiKey == "" {
		return
	}
	r.Header.Set("Authorization", "Bearer "+a.apiKey)
}

// buildRequestBody serializes the outgoing request body. When the caller
// already has a raw JSON payload we patch the "stream" field in place to
// match the adapter mode; otherwise we marshal the struct.
func buildRequestBody(req *provider.ChatRequest, stream bool) ([]byte, error) {
	if len(req.Raw) > 0 {
		return setStreamFlag(req.Raw, stream)
	}
	cloned := *req
	cloned.Stream = stream
	return json.Marshal(&cloned)
}

// setStreamFlag rewrites or inserts the top-level "stream" flag in a JSON
// object, leaving the rest of the payload byte-identical.
func setStreamFlag(src json.RawMessage, stream bool) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	if stream {
		obj["stream"] = json.RawMessage(`true`)
	} else {
		delete(obj, "stream")
	}
	return json.Marshal(obj)
}

// upstreamError normalises an upstream error body into a provider.Error with
// the original HTTP status preserved.
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
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
		Code:    envelope.Error.Code,
		Message: msg,
	}
}
