package types

import "time"

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
