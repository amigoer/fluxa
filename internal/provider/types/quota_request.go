package types

import "time"

type QuotaRequestStatus string

const (
	QuotaRequestPending  QuotaRequestStatus = "pending"
	QuotaRequestApproved QuotaRequestStatus = "approved"
	QuotaRequestRejected QuotaRequestStatus = "rejected"
)

type QuotaRequest struct {
	ID                  string
	RequestedByMemberID string
	ModelID             *string
	AmountCents         int64
	Reason              string
	Status              QuotaRequestStatus
	DecidedByMemberID   *string
	DecidedAt           *time.Time
	CreatedAt           time.Time
}
