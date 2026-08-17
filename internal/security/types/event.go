package types

import "time"

type SecurityEvent struct {
	ID           string
	MemberID     *string
	VirtualKeyID *string
	RuleID       *string
	Description  string
	ActionTaken  RuleAction
	OccurredAt   time.Time
}
