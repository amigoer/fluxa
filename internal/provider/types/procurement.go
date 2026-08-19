package types

import "time"

type ProcurementRecord struct {
	ID                 string
	ProviderID         string
	AmountMicroCents   int64
	Note               string
	RecordedByMemberID string
	RecordedAt         time.Time
}
