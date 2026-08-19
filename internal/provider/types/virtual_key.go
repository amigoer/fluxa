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
	ModelScope       []string
	BudgetMicroCents int64
	SpentMicroCents  int64
	// ReservedMicroCents is what in-flight calls have been promised but
	// have not yet settled. Admission is decided against
	// Budget - Spent - Reserved, so concurrent calls cannot each be told
	// there is room for the same money.
	ReservedMicroCents int64
	PeriodStartedAt    time.Time
	Status             VirtualKeyStatus
	CreatedAt          time.Time
	RevokedAt          *time.Time
}
