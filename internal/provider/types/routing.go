package types

import "time"

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
