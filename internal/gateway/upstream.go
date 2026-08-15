package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// chatMessage is the minimal OpenAI-compatible message shape: enough to
// pull the text DLP needs to scan, without modeling every field a real
// client might send.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	MaxTokens int           `json:"max_tokens"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatResponse struct {
	Usage chatUsage `json:"usage"`
}

// upstreamClient forwards a chat completion request to a provider and
// streams the response straight back to the caller, flushing every chunk
// as it arrives rather than buffering the whole thing -- required so DLP
// scanning the request doesn't come at the cost of also having to buffer
// (and thereby delay) the response (DESIGN.md 7.3 / section 5).
//
// Only openai_compatible providers are wired up to a real upstream call;
// the other kinds (anthropic, azure_openai, gemini, bedrock) are a
// reserved extension point, same status as the WeCom/DingTalk identity
// adapters -- each is a real vendor SDK/API translation of meaningful
// size that isn't implemented yet.
type upstreamClient struct {
	httpClient *http.Client
}

func newUpstreamClient() *upstreamClient {
	return &upstreamClient{httpClient: &http.Client{Timeout: 5 * time.Minute}}
}

// outcome is what forwarding a request produced, for the caller to
// account and log.
type outcome struct {
	// Usage is only populated for non-streaming calls, where the
	// provider's JSON response includes real token counts. Streaming
	// calls fall back to an estimate computed before the call, since
	// getting real usage would mean parsing the SSE stream instead of
	// just relaying it.
	Usage         *chatUsage
	StatusSuccess bool
}

func (c *upstreamClient) forward(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	switch p.Kind {
	case types.ProviderKindOpenAICompatible:
		return c.forwardOpenAICompatible(ctx, p, model, req, w)
	default:
		return outcome{}, fmt.Errorf("gateway: provider kind %q is not implemented yet", p.Kind)
	}
}

func (c *upstreamClient) forwardOpenAICompatible(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	baseURL, _ := p.Config["base_url"].(string)
	apiKey, _ := p.Config["api_key"].(string)
	if baseURL == "" {
		return outcome{}, fmt.Errorf("gateway: provider %q has no base_url configured", p.Name)
	}

	req.Model = model.ModelIdentifier
	body, err := json.Marshal(req)
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(upstreamReq)
	if err != nil {
		return outcome{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return outcome{StatusSuccess: false}, nil
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(http.StatusOK)

	if req.Stream {
		if _, err := io.Copy(flushWriter{w}, resp.Body); err != nil {
			return outcome{}, err
		}
		return outcome{StatusSuccess: true}, nil
	}

	// Non-streaming: the whole JSON body is read anyway to relay it, so
	// this is also where real usage numbers come from.
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{}, err
	}
	if _, err := w.Write(raw); err != nil {
		return outcome{}, err
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return outcome{StatusSuccess: true, Usage: &parsed.Usage}, nil
	}
	return outcome{StatusSuccess: true}, nil
}

// flushWriter flushes after every Write when the underlying
// ResponseWriter supports it, so a streamed response reaches the caller
// chunk by chunk instead of only once the whole thing has been copied.
type flushWriter struct {
	w http.ResponseWriter
}

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if flusher, ok := f.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
