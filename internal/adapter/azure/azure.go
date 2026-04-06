// Package azure implements provider.Provider on top of Azure OpenAI Service.
//
// Azure speaks the OpenAI chat-completions wire format but differs on three
// points: the path encodes a per-model deployment name, the api-version
// query parameter is mandatory, and authentication uses the "api-key" header
// instead of a bearer token. This adapter keeps the request/response bodies
// byte-identical to the OpenAI format so the router layer does not need to
// care which Azure deployment actually served the call.
package azure

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

// Adapter serves a single Azure OpenAI resource. A single Azure resource
// hosts multiple deployments (one per model), which is why we need the
// Deployments map at construction time.
type Adapter struct {
	name        string
	baseURL     string
	apiKey      string
	apiVersion  string
	deployments map[string]string
	headers     map[string]string
	client      *http.Client
	models      []string
}

// Options configures a new Azure Adapter.
type Options struct {
	Name       string
	BaseURL    string // e.g. "https://my-resource.openai.azure.com"
	APIKey     string
	APIVersion string // e.g. "2024-06-01"; defaults to "2024-06-01" when empty
	// Deployments maps canonical model identifiers to Azure deployment names.
	Deployments map[string]string
	Headers     map[string]string
	Timeout     time.Duration
}

// New builds an Azure Adapter. The caller is responsible for supplying a
// BaseURL; Azure has no sane default because every customer has their own
// resource hostname.
func New(opts Options) (*Adapter, error) {
	if opts.BaseURL == "" {
		return nil, errors.New("azure: base_url is required")
	}
	if opts.APIKey == "" {
		return nil, errors.New("azure: api_key is required")
	}
	if len(opts.Deployments) == 0 {
		return nil, errors.New("azure: deployments map is required")
	}
	version := opts.APIVersion
	if version == "" {
		version = "2024-06-01"
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	models := make([]string, 0, len(opts.Deployments))
	for m := range opts.Deployments {
		models = append(models, m)
	}

	return &Adapter{
		name:        opts.Name,
		baseURL:     strings.TrimRight(opts.BaseURL, "/"),
		apiKey:      opts.APIKey,
		apiVersion:  version,
		deployments: opts.Deployments,
		headers:     opts.Headers,
		models:      models,
		client:      &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the configured provider name.
func (a *Adapter) Name() string { return a.name }

// Models returns the set of canonical model identifiers this adapter knows
// how to route.
func (a *Adapter) Models() []string {
	out := make([]string, len(a.models))
	copy(out, a.models)
	return out
}

// Chat performs a non-streaming chat completion and relays the upstream
// OpenAI-shaped response body verbatim.
func (a *Adapter) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	body, err := buildRequestBody(req, false)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, body)
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
		Model string         `json:"model"`
		Usage provider.Usage `json:"usage"`
	}
	_ = json.Unmarshal(raw, &parsed)
	return &provider.ChatResponse{Model: parsed.Model, Usage: parsed.Usage, Raw: raw}, nil
}

// ChatStream opens an SSE connection and relays every chunk.
func (a *Adapter) ChatStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	body, err := buildRequestBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := a.do(ctx, req.Model, body)
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

// Health issues a cheap GET /openai/models probe to confirm reachability
// and credentials. The endpoint exists on every Azure OpenAI resource.
func (a *Adapter) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u := fmt.Sprintf("%s/openai/models?api-version=%s", a.baseURL, url.QueryEscape(a.apiVersion))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	a.applyAuth(req)
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

// do builds the deployment-scoped chat completions URL and dispatches the
// HTTP request. An unknown model returns a 404-style provider.Error so the
// caller can expose a useful message to the client.
func (a *Adapter) do(ctx context.Context, model string, body []byte) (*http.Response, error) {
	deployment, ok := a.deployments[model]
	if !ok {
		return nil, &provider.Error{
			Status:  http.StatusNotFound,
			Message: fmt.Sprintf("azure: no deployment configured for model %q", model),
		}
	}
	u := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		a.baseURL, url.PathEscape(deployment), url.QueryEscape(a.apiVersion))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	a.applyAuth(req)
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &provider.Error{Status: http.StatusBadGateway, Message: "upstream request failed", Cause: err}
	}
	return resp, nil
}

func (a *Adapter) applyAuth(r *http.Request) {
	r.Header.Set("api-key", a.apiKey)
}

// buildRequestBody serialises the outgoing body. Azure rejects requests that
// include a "model" field inside the body (the deployment lives in the URL),
// so we strip it when passing through a raw payload.
func buildRequestBody(req *provider.ChatRequest, stream bool) ([]byte, error) {
	if len(req.Raw) == 0 {
		cloned := *req
		cloned.Stream = stream
		cloned.Model = "" // Azure keeps the model in the URL.
		return json.Marshal(&cloned)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(req.Raw, &obj); err != nil {
		return nil, fmt.Errorf("azure: decode request body: %w", err)
	}
	delete(obj, "model")
	if stream {
		obj["stream"] = json.RawMessage(`true`)
	} else {
		delete(obj, "stream")
	}
	return json.Marshal(obj)
}

// upstreamError normalises an Azure error envelope into a provider.Error.
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
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
	return &provider.Error{Status: status, Code: envelope.Error.Code, Message: msg}
}
