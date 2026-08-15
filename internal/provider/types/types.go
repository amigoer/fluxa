// Package types holds the Provider module's domain entities on their
// own, with no further internal imports, specifically so that
// provider/health, provider/routing and provider/keyauth can depend on
// them without creating an import cycle back through the parent
// provider package (which is what wires those subpackages together in
// provider/service.go).
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

type ModelStatus string

const (
	ModelStatusDraft     ModelStatus = "draft"
	ModelStatusPublished ModelStatus = "published"
)

type Model struct {
	ID                    string
	ProviderID            string
	Name                  string
	ModelIdentifier       string
	Status                ModelStatus
	InputPriceCentsPer1M  int64
	OutputPriceCentsPer1M int64
	ContextWindow         int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ProcurementRecord struct {
	ID                 string
	ProviderID         string
	AmountCents        int64
	Note               string
	RecordedByMemberID string
	RecordedAt         time.Time
}

type RoutingScope string

const (
	RoutingScopeGlobal   RoutingScope = "global"
	RoutingScopePersonal RoutingScope = "personal"
)

// RoutingRule is one step of a fallback chain: try TargetModelID, and if
// it fails (or its provider is circuit-open) fall through to
// FallbackModelID. Global rules are admin-owned baseline routing;
// personal rules are an employee's own configuration layered on top
// (DESIGN.md 7.2). CostCeilingCents, when set, caps how much the chain
// is allowed to spend attempting fallbacks for a single request.
type RoutingRule struct {
	ID               string
	Scope            RoutingScope
	OwnerMemberID    *string
	ConditionLabel   string
	TargetModelID    string
	FallbackModelID  *string
	CostCeilingCents *int64
	SortOrder        int
	CreatedAt        time.Time
}

type VirtualKeyOwnerType string

const (
	VirtualKeyOwnerMember     VirtualKeyOwnerType = "member"
	VirtualKeyOwnerDepartment VirtualKeyOwnerType = "department"
)

type VirtualKeyStatus string

const (
	VirtualKeyStatusActive  VirtualKeyStatus = "active"
	VirtualKeyStatusRevoked VirtualKeyStatus = "revoked"
)

// VirtualKey is the quota carrier every programmatic call authenticates
// with (DESIGN.md 7.2). SecretHash is sha256(raw secret); the raw value
// is only ever returned once, at creation time.
type VirtualKey struct {
	ID                string
	Name              string
	SecretHash        string
	SecretPrefix      string
	OwnerType         VirtualKeyOwnerType
	OwnerMemberID     *string
	OwnerDepartmentID *string
	// nil means every enabled model is in scope.
	ModelScope      []string
	BudgetCents     int64
	SpentCents      int64
	PeriodStartedAt time.Time
	Status          VirtualKeyStatus
	CreatedAt       time.Time
	RevokedAt       *time.Time
}

type QuotaRequestStatus string

const (
	QuotaRequestPending  QuotaRequestStatus = "pending"
	QuotaRequestApproved QuotaRequestStatus = "approved"
	QuotaRequestRejected QuotaRequestStatus = "rejected"
)

type QuotaRequest struct {
	ID                  string
	RequestedByMemberID string
	ModelID             *string
	AmountCents         int64
	Reason              string
	Status              QuotaRequestStatus
	DecidedByMemberID   *string
	DecidedAt           *time.Time
	CreatedAt           time.Time
}

type HealthState string

const (
	HealthStateNormal      HealthState = "normal"
	HealthStateCircuitOpen HealthState = "circuit_open"
	HealthStateHalfOpen    HealthState = "half_open"
)

// ProviderHealth tracks the circuit breaker state machine described in
// DESIGN.md 3/7.2: normal -> circuit_open after too many consecutive
// failures -> half_open after a cooldown to probe recovery -> back to
// normal on a successful probe, or circuit_open again on a failed one.
type ProviderHealth struct {
	ProviderID          string
	State               HealthState
	ConsecutiveFailures int
	OpenedAt            *time.Time
	LastProbeAt         *time.Time
	UpdatedAt           time.Time
}
