// Package audit implements call logs (every proxied request) and
// operation audit logs (admin actions), kept as two separate tables per
// DESIGN.md's v2 roadmap note distinguishing them.
package audit

import "time"

type CallStatus string

const (
	CallStatusSuccess CallStatus = "success"
	CallStatusFailed  CallStatus = "failed"
)

type CallLog struct {
	ID           string
	MemberID     string
	VirtualKeyID string
	ProviderID   string
	ModelID      string
	RequestID    string
	Status       CallStatus
	LatencyMS    int
	InputTokens  int
	OutputTokens int
	CostCents    int64
	OccurredAt   time.Time
}

type OperationAuditLog struct {
	ID            string
	ActorMemberID string
	Action        string
	Detail        string
	OccurredAt    time.Time
}
