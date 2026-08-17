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
