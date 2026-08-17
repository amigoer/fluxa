package types

import "time"

type MemberStatus string

const (
	MemberStatusActive        MemberStatus = "active"
	MemberStatusPendingReview MemberStatus = "pending_review"
	MemberStatusDisabled      MemberStatus = "disabled"
)

type Member struct {
	ID           string
	OrgID        string
	DepartmentID *string
	RoleID       string
	Name         string
	Email        *string
	Phone        *string
	// AvatarURL is the identity source's picture of this member, nil for
	// anyone who signed up locally and has no source to take one from.
	AvatarURL *string
	Status    MemberStatus
	CreatedAt time.Time
}
