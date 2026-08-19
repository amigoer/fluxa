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

// fakeUpstream stands in for an OpenAI-compatible provider and records
// the body it was sent.
func fakeUpstream(t *testing.T, respond func(w http.ResponseWriter, body map[string]any)) (types.Provider, *map[string]any) {
	t.Helper()
	received := map[string]any{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &received); err != nil {
			t.Errorf("upstream got a body it could not parse: %v", err)
		}
		respond(w, received)
	}))
	t.Cleanup(srv.Close)

	return types.Provider{
		Name: "fake",
		Kind: types.ProviderKindOpenAICompatible,
		Config: map[string]any{
			"base_url": srv.URL,
			"api_key":  "sk-test",
		},
	}, &received
}

func streamingModel() types.Model {
	return types.Model{ModelIdentifier: "gpt-4o-2024-11-20", InputPriceCentsPer1M: 1000, OutputPriceCentsPer1M: 2000}
}

// The whole point of P1-3: a streamed call now comes back with the
// provider's real token counts instead of nothing.
func TestForwardStreamingReturnsRealUsage(t *testing.T) {
	provider, received := fakeUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, chunk(deltaFrame)+chunk(usageFrame)+chunk("[DONE]"))
	})

	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec := httptest.NewRecorder()
	got, err := newUpstreamClient().forward(context.Background(), provider, streamingModel(), req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if got.Usage == nil {
		t.Fatal("streaming call came back with no usage")
	}
	if got.Usage.PromptTokens != 1200 || got.Usage.CompletionTokens != 340 {
		t.Errorf("usage = %+v, want the provider's 1200/340", *got.Usage)
	}

	// It was asked for, and the caller who did not ask does not see it.
	opts, _ := (*received)["stream_options"].(map[string]any)
	if opts["include_usage"] != true {
		t.Errorf("stream_options sent upstream = %v, want include_usage true", (*received)["stream_options"])
	}
	if body := rec.Body.String(); strings.Contains(body, "usage") {
		t.Errorf("the usage frame was relayed to a caller who never asked for it:\n%s", body)
	}
	if body, want := rec.Body.String(), chunk(deltaFrame)+chunk("[DONE]"); body != want {
		t.Errorf("relayed:\n%q\nwant:\n%q", body, want)
	}
}

// A provider configured to opt out is not sent the option at all, and
// its streamed calls go back to being estimated.
func TestForwardRespectsAProviderThatOptsOutOfStreamUsage(t *testing.T) {
	provider, received := fakeUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, chunk(deltaFrame)+chunk("[DONE]"))
	})
	provider.Config["disable_stream_usage"] = true

	req, err := decodeChatRequest(strings.NewReader(
		`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec := httptest.NewRecorder()
	got, err := newUpstreamClient().forward(context.Background(), provider, streamingModel(), req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if _, sent := (*received)["stream_options"]; sent {
		t.Error("stream_options was sent to a provider that opted out")
	}
	if got.Usage != nil {
		t.Errorf("usage = %+v, want nil so the caller falls back to an estimate", *got.Usage)
	}
}

func TestForwardNonStreamingStillReadsUsageFromTheBody(t *testing.T) {
	provider, _ := fakeUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[],"usage":{"prompt_tokens":50,"completion_tokens":20}}`)
	})

	req, err := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec := httptest.NewRecorder()
	got, err := newUpstreamClient().forward(context.Background(), provider, streamingModel(), req, rec)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if got.Usage == nil || got.Usage.PromptTokens != 50 || got.Usage.CompletionTokens != 20 {
		t.Errorf("usage = %+v, want 50/20", got.Usage)
	}
}

// The five kinds the schema allows are all implemented now. A kind that
// is not still has to fail here rather than be attempted -- this and
// types.ProviderKind.Implemented are the two halves of that guarantee,
// and they have to be changed together.
func TestForwardRefusesAProviderKindItCannotSpeak(t *testing.T) {
	provider := types.Provider{Kind: types.ProviderKind("cohere"), Config: map[string]any{"base_url": "http://127.0.0.1:1"}}
	_, err := newUpstreamClient().forward(context.Background(), provider, streamingModel(), chatRequest{}, httptest.NewRecorder())
	if err == nil {
		t.Fatal("forward accepted a provider kind with no implementation behind it")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("err = %v, want it to say the kind is not implemented", err)
	}
}

// Every kind the console can save must have a case in forward. A kind
// that Implemented() waves through but forward cannot speak is exactly
// the configurable-but-not-callable state this pair exists to prevent.
func TestEveryImplementedKindIsReachableFromForward(t *testing.T) {
	for _, kind := range []types.ProviderKind{
		types.ProviderKindOpenAICompatible,
		types.ProviderKindAzureOpenAI,
		types.ProviderKindAnthropic,
		types.ProviderKindGemini,
		types.ProviderKindBedrock,
	} {
		if !kind.Implemented() {
			t.Errorf("%q is not marked implemented", kind)
			continue
		}
		// Point at a closed port: reaching a dial error proves the
		// dispatch found a case, where a missing case fails earlier with
		// "not implemented".
		provider := types.Provider{
			Kind: kind,
			Config: map[string]any{
				"base_url":          "http://127.0.0.1:1",
				"region":            "us-east-1",
				"access_key_id":     "AKID",
				"secret_access_key": "secret",
			},
		}
		req, _ := decodeChatRequest(strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
		_, err := newUpstreamClient().forward(context.Background(), provider, streamingModel(), req, httptest.NewRecorder())
		if err != nil && strings.Contains(err.Error(), "not implemented") {
			t.Errorf("%q is saveable but forward has no case for it", kind)
		}
	}
}
