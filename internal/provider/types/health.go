package types

import "time"

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
