// Package provider defines the unified interface that every upstream model
// Provider implements. The router layer only talks to this interface, which
// keeps adapter implementations isolated and makes new providers trivial to
// add.
package provider

import (
	"context"
	"encoding/json"
	"errors"
)

// Provider is the contract every upstream backend must satisfy.
//
// Implementations are expected to be safe for concurrent use by multiple
// goroutines because a single instance is shared across all in-flight
// requests.
type Provider interface {
	// Name returns the stable identifier used in configuration and logs.
	Name() string

	// Chat executes a non-streaming chat completion.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// ChatStream executes a streaming chat completion. Implementations MUST
	// close the returned channel when the stream ends, either because the
	// upstream finished or because the context was cancelled.
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// Models returns the list of model identifiers this provider serves.
	Models() []string

	// Health performs a lightweight check that the provider is reachable and
	// the credentials are valid. It should return quickly and never block on
	// a full completion.
	Health(ctx context.Context) error
}

// Role identifies the author of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in a chat conversation. Content is stored as raw
// JSON so adapters can pass through rich multimodal payloads (text + image
// parts) without lossy intermediate representations.
type Message struct {
	Role    Role            `json:"role"`
	Name    string          `json:"name,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// ChatRequest is the provider-neutral representation of a chat completion
// request. Adapters translate it into the wire format of their target API.
type ChatRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Stream           bool      `json:"stream,omitempty"`
	Temperature      *float64  `json:"temperature,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	User             string    `json:"user,omitempty"`

	// Raw carries the untouched inbound JSON body. Adapters that share the
	// OpenAI wire format can forward it verbatim to preserve every field
	// (tool_calls, response_format, logprobs, etc.) without having to keep
	// this struct in lock-step with upstream changes.
	Raw json.RawMessage `json:"-"`
}

// ChatResponse is a non-streaming completion result. The Raw field carries
// the adapter's already-translated OpenAI-compatible response so handlers can
// ship it to clients without another round of serialization.
type ChatResponse struct {
	Model string
	Usage Usage
	Raw   json.RawMessage
}

// Usage reports token accounting for a completed request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamEvent is a single chunk delivered by ChatStream. Exactly one of Data
// and Err is populated. Data is the serialized SSE payload in OpenAI format
// (the JSON object only, without the "data: " prefix or trailing newline).
type StreamEvent struct {
	Data []byte
	Err  error
}

// ErrNoProvider is returned by the router when no provider can serve a model.
var ErrNoProvider = errors.New("no provider available for model")

// Error is a provider-side error that carries an HTTP status code so handlers
// can surface the original upstream status to clients.
type Error struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (e *Error) Unwrap() error { return e.Cause }
