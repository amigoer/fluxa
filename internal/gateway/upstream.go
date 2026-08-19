package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// errContentParts is a message whose content is an array of parts
// (OpenAI's multimodal shape) rather than a string. Relaying it
// untouched would mean DLP never sees the text inside it, so it is
// refused outright rather than passed through unscanned; lifting this
// means teaching the scanner to pull the text parts out and put them
// back, not deleting the check.
var errContentParts = errors.New("gateway: message content must be a string; content parts are not supported yet")

// chatMessage is one entry of a request's messages array. The gateway
// reads exactly two fields off it -- role, and content for DLP to scan
// and mask -- and holds on to the object they were parsed from, so
// putting the masked content back cannot drop the fields the gateway
// deliberately does not model (name, tool_calls, tool_call_id, and
// whatever a provider adds next).
type chatMessage struct {
	raw map[string]json.RawMessage

	Role    string
	Content string

	// hasTextContent separates a message whose content is a string from
	// one that omitted it or sent an explicit null -- an assistant turn
	// carrying only tool_calls does the latter, and rewriting that null
	// to "" changes what the provider is being told.
	hasTextContent bool
}

func (m *chatMessage) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &m.raw); err != nil {
		return err
	}
	if v, ok := m.raw["role"]; ok {
		if err := json.Unmarshal(v, &m.Role); err != nil {
			return fmt.Errorf("gateway: message role: %w", err)
		}
	}
	if v, ok := m.raw["content"]; ok {
		var content *string
		if err := json.Unmarshal(v, &content); err != nil {
			return errContentParts
		}
		if content != nil {
			m.Content = *content
			m.hasTextContent = true
		}
	}
	return nil
}

func (m chatMessage) MarshalJSON() ([]byte, error) {
	out := make(map[string]json.RawMessage, len(m.raw))
	for k, v := range m.raw {
		out[k] = v
	}
	if m.hasTextContent {
		content, err := json.Marshal(m.Content)
		if err != nil {
			return nil, err
		}
		out["content"] = content
	}
	return json.Marshal(out)
}

// chatRequest is a caller's /v1/chat/completions body: the four fields
// the gateway acts on, plus the object they were parsed from.
//
// The upstream body is rebuilt from raw rather than re-marshalled from
// the parsed fields, and that is the whole point of keeping raw around.
// Re-marshalling a narrow struct silently dropped every parameter the
// struct didn't happen to list -- temperature, top_p, stop, seed,
// response_format, stream_options, and, worst of all, tools and
// tool_choice, so function calling quietly stopped working instead of
// failing -- and just as silently injected "max_tokens": 0 into every
// request from a caller who never set one.
type chatRequest struct {
	raw map[string]json.RawMessage

	Model     string
	Messages  []chatMessage
	Stream    bool
	MaxTokens int
}

func decodeChatRequest(r io.Reader) (chatRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return chatRequest{}, err
	}

	req := chatRequest{raw: raw}
	if err := decodeField(raw, "model", &req.Model); err != nil {
		return chatRequest{}, err
	}
	if err := decodeField(raw, "messages", &req.Messages); err != nil {
		return chatRequest{}, err
	}
	if err := decodeField(raw, "stream", &req.Stream); err != nil {
		return chatRequest{}, err
	}
	if err := decodeField(raw, "max_tokens", &req.MaxTokens); err != nil {
		return chatRequest{}, err
	}
	return req, nil
}

// decodeField reads one field out of an already-parsed body into target,
// leaving target at its zero value when the caller omitted the field or
// sent an explicit null.
func decodeField(raw map[string]json.RawMessage, name string, target any) error {
	v, ok := raw[name]
	if !ok || string(v) == "null" {
		return nil
	}
	if err := json.Unmarshal(v, target); err != nil {
		if errors.Is(err, errContentParts) {
			return err
		}
		return fmt.Errorf("gateway: field %q: %w", name, err)
	}
	return nil
}

// wantsUsageFrame reports whether the caller itself asked for the
// trailing usage frame, via stream_options.include_usage. When it did,
// that frame is its own and is relayed; when it did not, the gateway
// asks for the frame anyway to bill from and then hides it again.
func (req chatRequest) wantsUsageFrame() bool {
	raw, ok := req.raw["stream_options"]
	if !ok {
		return false
	}
	var opts struct {
		IncludeUsage bool `json:"include_usage"`
	}
	if err := json.Unmarshal(raw, &opts); err != nil {
		return false
	}
	return opts.IncludeUsage
}

// bodyOptions are the changes forwarding makes to a caller's request on
// its way upstream.
type bodyOptions struct {
	// ModelIdentifier is the provider's own name for whichever model
	// routing picked, which replaces the name the caller used.
	ModelIdentifier string

	// RequestUsage sets stream_options.include_usage, so a streamed
	// response ends with a frame reporting real token counts. Without it
	// the gateway has nothing to bill a streamed call from but an
	// estimate.
	RequestUsage bool
}

// body renders what actually goes upstream: the caller's own object,
// with model swapped for the provider's identifier, messages swapped for
// the DLP-masked ones, and stream_options set when usage is wanted.
// Every other key is relayed unchanged, including ones this version of
// Fluxa has never heard of -- re-encoded, so insignificant whitespace
// inside a value is not preserved, but never reinterpreted or dropped.
func (req chatRequest) body(opts bodyOptions) ([]byte, error) {
	out := make(map[string]json.RawMessage, len(req.raw)+3)
	for k, v := range req.raw {
		out[k] = v
	}

	model, err := json.Marshal(opts.ModelIdentifier)
	if err != nil {
		return nil, err
	}
	out["model"] = model

	if req.Messages != nil {
		messages, err := json.Marshal(req.Messages)
		if err != nil {
			return nil, err
		}
		out["messages"] = messages
	}

	if opts.RequestUsage {
		streamOptions, err := withIncludeUsage(req.raw["stream_options"])
		if err != nil {
			return nil, err
		}
		out["stream_options"] = streamOptions
	}

	return json.Marshal(out)
}

// withIncludeUsage turns include_usage on inside the caller's own
// stream_options rather than replacing the object, so any other option
// it set there survives.
func withIncludeUsage(existing json.RawMessage) (json.RawMessage, error) {
	opts := map[string]json.RawMessage{}
	if len(existing) > 0 && string(existing) != "null" {
		if err := json.Unmarshal(existing, &opts); err != nil {
			// The caller sent something that is not an object. Replacing
			// it would discard whatever they meant, so leave it alone and
			// bill from the estimate instead.
			return existing, nil
		}
	}
	opts["include_usage"] = json.RawMessage("true")
	return json.Marshal(opts)
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatResponse struct {
	Usage chatUsage `json:"usage"`
}

// upstreamClient forwards a chat completion request to a provider and
// streams the response back to the caller frame by frame, flushing each
// one as it arrives rather than buffering the whole thing -- required so
// DLP scanning the request doesn't come at the cost of also having to
// buffer (and thereby delay) the response (DESIGN.md 7.3 / section 5).
// A streamed response is read on the way past for the token usage the
// call is billed from (see sseRelay), which costs one frame of buffering
// and no delay.
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
	// Usage is the token counts the provider reported: read out of the
	// response body for a non-streaming call, out of the stream's final
	// frame for a streaming one. It is nil only when the provider
	// reported none -- an older OpenAI-compatible server that ignores
	// stream_options, or one this deployment opted out of asking -- and
	// the call then falls back to being billed on an estimate.
	Usage         *chatUsage
	StatusSuccess bool
}

// providerReportsStreamUsage reports whether to ask this provider for
// usage on a streamed call.
//
// It is on by default because it is the only way to bill a streamed call
// from real numbers. The escape hatch exists because stream_options is a
// comparatively recent addition to the OpenAI API, and a strict
// self-hosted server predating it may reject the whole request over an
// unknown field -- which would take the call down rather than just the
// usage. Set "disable_stream_usage": true in such a provider's config
// and its streamed calls go back to being estimated.
func providerReportsStreamUsage(p types.Provider) bool {
	switch v := p.Config["disable_stream_usage"].(type) {
	case bool:
		return !v
	case string:
		return v != "true"
	default:
		return true
	}
}

// forward sends a request to whichever upstream the routing decision
// picked, translating to and from that vendor's wire format as needed
// and relaying the response to w in OpenAI chat-completions shape --
// which is the only shape callers of this endpoint ever see, whoever
// actually served the request.
//
// One file per vendor sits beside this one. Adding a vendor means adding
// a case here, a file next to it, and the kind to
// types.ProviderKind.Implemented -- which is what stops a provider being
// configurable before it is callable.
func (c *upstreamClient) forward(ctx context.Context, p types.Provider, model types.Model, req chatRequest, w http.ResponseWriter) (outcome, error) {
	switch p.Kind {
	case types.ProviderKindOpenAICompatible, types.ProviderKindAzureOpenAI:
		return c.forwardOpenAIFormat(ctx, p, model, req, w)
	case types.ProviderKindAnthropic:
		return c.forwardAnthropic(ctx, p, model, req, w)
	case types.ProviderKindGemini:
		return c.forwardGemini(ctx, p, model, req, w)
	case types.ProviderKindBedrock:
		return c.forwardBedrock(ctx, p, model, req, w)
	default:
		return outcome{}, fmt.Errorf("gateway: provider kind %q is not implemented yet", p.Kind)
	}
}

// relayUpstreamError copies a provider's own error response through to
// the caller unchanged. A gateway that rewrites upstream errors hides
// the one thing that says what went wrong -- a rate limit, a bad key, a
// context-length overflow all arrive as the vendor described them.
func relayUpstreamError(w http.ResponseWriter, resp *http.Response) (outcome, error) {
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return outcome{StatusSuccess: false}, nil
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
