package types

import "time"

type Department struct {
	ID           string
	OrgID        string
	Name         string
	LeadMemberID *string
	CreatedAt    time.Time
}
