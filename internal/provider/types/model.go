package types

import "time"

type ModelStatus string

const (
	ModelStatusDraft     ModelStatus = "draft"
	ModelStatusPublished ModelStatus = "published"
)

type Model struct {
	ID                    string
	ProviderID            string
	Name                  string
	ModelIdentifier       string
	Status                ModelStatus
	InputPriceCentsPer1M  int64
	OutputPriceCentsPer1M int64
	ContextWindow         int
	// ProviderKind is joined in by every query that returns models to the
	// frontend (ListModels, ListPublishedModels), so a model picker can
	// render its vendor mark without a separate /api/providers call --
	// which an employee isn't permitted to make anyway. Zero value only
	// on GetModel, which serves the routing resolver and never the UI.
	ProviderKind ProviderKind
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
