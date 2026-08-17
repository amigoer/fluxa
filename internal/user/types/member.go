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
	Status       MemberStatus
	CreatedAt    time.Time
}
