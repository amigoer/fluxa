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

	"github.com/amigoer/fluxa/internal/provider/types"
)

// This file serves every provider that already speaks the OpenAI
// chat-completions wire format: openai_compatible (OpenAI itself and the
// many vendors that copied its API -- Kimi, GLM, Doubao, Qwen, DeepSeek,
// a self-hosted vLLM) and azure_openai.
//
// Azure is here rather than in a file of its own because the bodies are
// byte-identical to OpenAI's. It differs on exactly three points: the
// deployment name is in the path, an api-version query parameter is
// mandatory, and the credential goes in an api-key header instead of a
// bearer token.

// defaultAzureAPIVersion is used when a provider's config does not pin
// one. Azure rejects a request with no api-version at all.
const defaultAzureAPIVersion = "2024-10-21"

// openAIEndpoint builds the URL to POST to.
func openAIEndpoint(p types.Provider, model types.Model) (string, error) {
	baseURL, _ := p.Config["base_url"].(string)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("gateway: provider %q has no base_url configured", p.Name)
	}

	if p.Kind != types.ProviderKindAzureOpenAI {
		return baseURL + "/chat/completions", nil
	}

	// On Azure the model identifier names a deployment, and the
	// deployment is part of the path rather than the body.
	apiVersion, _ := p.Config["api_version"].(string)
	if apiVersion == "" {
		apiVersion = defaultAzureAPIVersion
	}
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		baseURL, url.PathEscape(model.ModelIdentifier), url.QueryEscape(apiVersion)), nil
}

// openAIEmbeddingsEndpoint is openAIEndpoint's sibling for the
// embeddings path, which differs only in the last segment.
func openAIEmbeddingsEndpoint(p types.Provider, model types.Model) (string, error) {
	endpoint, err := openAIEndpoint(p, model)
	if err != nil {
		return "", err
	}
	return strings.Replace(endpoint, "/chat/completions", "/embeddings", 1), nil
}

// applyOpenAIAuth sets the credential header this vendor expects.
func applyOpenAIAuth(r *http.Request, p types.Provider) {
	apiKey, _ := p.Config["api_key"].(string)
	if apiKey == "" {
		return
	}
	if p.Kind == types.ProviderKindAzureOpenAI {
		r.Header.Set("api-key", apiKey)
		return
	}
	r.Header.Set("Authorization", "Bearer "+apiKey)
}

func (c *upstreamClient) forwardOpenAIFormat(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	endpoint, err := openAIEndpoint(p, model)
	if err != nil {
		return outcome{}, err
	}

	// Ask for usage on every streamed call, whether or not the caller
	// did -- it is what the call gets billed from. A provider that
	// rejects the option can opt out per-provider; see
	// providerReportsStreamUsage.
	callerWantsUsageFrame := req.wantsUsageFrame()
	requestUsage := req.Stream && providerReportsStreamUsage(p)

	body, err := req.body(bodyOptions{
		ModelIdentifier: model.ModelIdentifier,
		RequestUsage:    requestUsage,
	})
	if err != nil {
		return outcome{}, err
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return outcome{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	applyOpenAIAuth(upstreamReq, p)

	resp, err := c.httpClient.Do(upstreamReq)
	if err != nil {
		return outcome{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return relayUpstreamError(w, resp)
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(http.StatusOK)

	if req.Stream {
		// Relayed frame by frame rather than copied wholesale, so the
		// usage the provider reports at the end is actually read. The
		// caller sees the same stream at the same rate either way.
		relay := &sseRelay{DropUsageOnlyFrame: requestUsage && !callerWantsUsageFrame}
		if err := relay.Run(flushWriter{w}, resp.Body); err != nil {
			return outcome{}, err
		}
		return outcome{StatusSuccess: true, Usage: relay.Usage}, nil
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
