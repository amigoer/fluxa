package provider

import "context"

// MessagesRequest is the provider-neutral envelope for an Anthropic-style
// /v1/messages call. Like ChatRequest we keep the original body around so
// adapters that speak Anthropic natively can forward it untouched.
type MessagesRequest struct {
	Model string `json:"model"`
	// Raw is the full JSON body as received by the gateway handler.
	Raw []byte `json:"-"`
}

// MessagesResponse is a non-streaming /v1/messages result. Raw is the
// upstream JSON body, ready to ship back to the client.
type MessagesResponse struct {
	Model string
	Usage Usage
	Raw   []byte
}

// MessagesProvider is implemented by providers that speak the Anthropic
// Messages API natively. Dispatching /v1/messages through this interface
// keeps the wire format byte-identical to the upstream so clients like
// Claude Code get the thinking / tool_use blocks they expect.
type MessagesProvider interface {
	Provider
	Messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error)
	MessagesStream(ctx context.Context, req *MessagesRequest) (<-chan StreamEvent, error)
}
