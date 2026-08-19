package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

func TestEmbeddingsReadsEveryInputShape(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"a single string", `{"model":"e","input":"hello"}`, []string{"hello"}},
		{"an array of strings", `{"model":"e","input":["a","b"]}`, []string{"a", "b"}},
		// Pre-tokenized input carries no text for DLP to read.
		{"token ids", `{"model":"e","input":[[1,2,3]]}`, nil},
	} {
		req, err := decodeEmbeddingsRequest(strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		got := req.texts()
		if len(got) != len(tc.want) {
			t.Errorf("%s: texts = %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: texts[%d] = %q, want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

func TestEmbeddingsRequiresInput(t *testing.T) {
	if _, err := decodeEmbeddingsRequest(strings.NewReader(`{"model":"e"}`)); err == nil {
		t.Error("a request with no input was accepted")
	}
}

// Masked text goes back in the shape it came out of, and everything else
// the caller sent is preserved -- the same rule the chat path follows.
func TestEmbeddingsPutsMaskedTextBackAndKeepsOtherFields(t *testing.T) {
	req, err := decodeEmbeddingsRequest(strings.NewReader(
		`{"model":"e","input":["id 110101199003078515","plain"],"encoding_format":"base64","dimensions":256,"user":"emp-1"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	raw, err := req.withTexts([]string{"id 1****************5", "plain"}, "text-embedding-3-small")
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if string(out["input"]) != `["id 1****************5","plain"]` {
		t.Errorf("input = %s", out["input"])
	}
	if string(out["model"]) != `"text-embedding-3-small"` {
		t.Errorf("model = %s, want the provider's identifier", out["model"])
	}
	for field, want := range map[string]string{
		"encoding_format": `"base64"`,
		"dimensions":      `256`,
		"user":            `"emp-1"`,
	} {
		if string(out[field]) != want {
			t.Errorf("%s = %s, want %s", field, out[field], want)
		}
	}
}

// A string input stays a string on the way out; turning it into a
// one-element array is a different request.
func TestEmbeddingsKeepsAStringInputAString(t *testing.T) {
	req, _ := decodeEmbeddingsRequest(strings.NewReader(`{"model":"e","input":"hello"}`))
	raw, err := req.withTexts([]string{"masked"}, "m")
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var out map[string]json.RawMessage
	_ = json.Unmarshal(raw, &out)
	if string(out["input"]) != `"masked"` {
		t.Errorf("input = %s, want a bare string", out["input"])
	}
}

func TestForwardEmbeddingsRelaysAndReadsUsage(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = io.WriteString(w, `{"object":"list","data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":8,"total_tokens":8}}`)
	}))
	t.Cleanup(srv.Close)

	provider := types.Provider{
		Kind:   types.ProviderKindOpenAICompatible,
		Config: map[string]any{"base_url": srv.URL + "/v1", "api_key": "sk-test"},
	}
	req, _ := decodeEmbeddingsRequest(strings.NewReader(`{"model":"e","input":"hello"}`))
	rec := httptest.NewRecorder()

	got, err := newUpstreamClient().forwardEmbeddings(context.Background(), provider,
		types.Model{ModelIdentifier: "text-embedding-3-small"}, req, []string{"hello"}, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if gotPath != "/v1/embeddings" {
		t.Errorf("path = %q, want the embeddings endpoint", gotPath)
	}
	if gotBody["model"] != "text-embedding-3-small" {
		t.Errorf("model sent = %v", gotBody["model"])
	}
	if got.Usage == nil || got.Usage.PromptTokens != 8 {
		t.Errorf("usage = %+v, want 8 prompt tokens", got.Usage)
	}
	if !strings.Contains(rec.Body.String(), "embedding") {
		t.Errorf("response was not relayed: %s", rec.Body.String())
	}
}

// Gemini, Bedrock and Anthropic embeddings are different APIs (or, for
// Anthropic, none at all). Saying so beats failing somewhere unhelpful.
func TestForwardEmbeddingsRefusesProvidersThatCannotServeThem(t *testing.T) {
	for _, kind := range []types.ProviderKind{
		types.ProviderKindAnthropic,
		types.ProviderKindGemini,
		types.ProviderKindBedrock,
	} {
		provider := types.Provider{Kind: kind, Config: map[string]any{"base_url": "http://127.0.0.1:1"}}
		req, _ := decodeEmbeddingsRequest(strings.NewReader(`{"model":"e","input":"hi"}`))
		_, err := newUpstreamClient().forwardEmbeddings(context.Background(), provider,
			types.Model{}, req, []string{"hi"}, httptest.NewRecorder())
		if err == nil {
			t.Errorf("%q accepted an embeddings request it cannot serve", kind)
			continue
		}
		if !strings.Contains(err.Error(), "embeddings") {
			t.Errorf("%q: err = %v, want it to name the limitation", kind, err)
		}
	}
}

func TestEmbeddingsUsesTheAzureEndpointShape(t *testing.T) {
	provider := types.Provider{
		Kind:   types.ProviderKindAzureOpenAI,
		Config: map[string]any{"base_url": "https://r.openai.azure.com", "api_version": "2024-10-21"},
	}
	got, err := openAIEmbeddingsEndpoint(provider, types.Model{ModelIdentifier: "my-deployment"})
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	const want = "https://r.openai.azure.com/openai/deployments/my-deployment/embeddings?api-version=2024-10-21"
	if got != want {
		t.Errorf("endpoint = %q\nwant       %q", got, want)
	}
}
