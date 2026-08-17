package types

import "time"

type OperationAuditLog struct {
	ID            string
	ActorMemberID string
	Action        string
	Detail        string
	OccurredAt    time.Time
}
