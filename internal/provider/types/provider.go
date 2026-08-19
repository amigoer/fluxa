// Package types holds the Provider module's domain entities on their
// own, with no further internal imports, specifically so that
// provider/health, provider/routing and provider/keyauth can depend on
// them without creating an import cycle back through the parent
// provider package (which is what wires those subpackages together in
// provider/service). One entity per file, named to match the repo,
// service and handler file that works with it.
package types

import "time"

type ProviderKind string

const (
	ProviderKindOpenAICompatible ProviderKind = "openai_compatible"
	ProviderKindAnthropic        ProviderKind = "anthropic"
	ProviderKindAzureOpenAI      ProviderKind = "azure_openai"
	ProviderKindGemini           ProviderKind = "gemini"
	ProviderKindBedrock          ProviderKind = "bedrock"
)

// Implemented reports whether the gateway can actually forward a request
// to this kind of provider.
//
// It exists because the schema's CHECK constraint and the gateway's
// ability to speak a protocol are two different things, and for a while
// they disagreed: every kind was saveable and only one was callable, so
// a provider could look configured and healthy in the console and fail
// every single call at runtime. This is the gate that keeps a kind from
// being configurable before it is callable.
//
// It mirrors the switch in gateway.upstreamClient.forward; adding a
// vendor means changing both.
func (k ProviderKind) Implemented() bool {
	switch k {
	case ProviderKindOpenAICompatible, ProviderKindAzureOpenAI,
		ProviderKindAnthropic, ProviderKindGemini, ProviderKindBedrock:
		return true
	default:
		return false
	}
}

type ProviderStatus string

const (
	ProviderStatusActive   ProviderStatus = "active"
	ProviderStatusDisabled ProviderStatus = "disabled"
)

type Provider struct {
	ID        string
	OrgID     string
	Name      string
	Kind      ProviderKind
	Config    map[string]any
	Status    ProviderStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
