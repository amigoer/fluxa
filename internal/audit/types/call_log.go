// Package types holds the Audit module's domain entities: call logs
// (every proxied request) and operation audit logs (admin actions), kept
// as two separate tables per DESIGN.md's v2 roadmap note distinguishing
// them.
//
// It imports nothing else from this module, so audit/repo, audit/service
// and audit/handler -- and the gateway, which records call logs as it
// proxies -- can all depend on it without an import cycle. One entity
// per file, named to match the repo and service file that works with it.
package types

import "time"

type CallStatus string

const (
	CallStatusSuccess CallStatus = "success"
	CallStatusFailed  CallStatus = "failed"
)

type CallLog struct {
	ID             string
	MemberID       string
	VirtualKeyID   string
	ProviderID     string
	ModelID        string
	RequestID      string
	Status         CallStatus
	LatencyMS      int
	InputTokens    int
	OutputTokens   int
	CostMicroCents int64
	OccurredAt     time.Time
}
