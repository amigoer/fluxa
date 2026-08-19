// Package gateway is the request-proxying runtime path described in
// DESIGN.md 4: authenticate -> DLP filter (request only) -> routing
// decision -> call the provider, streaming the response back -> record
// spend -> write the audit trail. This is the actual "gateway" in
// Fluxa's name; everything else in the codebase is management UI around
// it.
package gateway

import (
	"github.com/go-chi/chi/v5"

	auditservice "github.com/amigoer/fluxa/internal/audit/service"
	providerservice "github.com/amigoer/fluxa/internal/provider/service"
	securityservice "github.com/amigoer/fluxa/internal/security/service"
)

// Pipeline is the gateway's assembly point. The route table lives here
// and nothing else does: the endpoint itself is in chat_completions.go
// and the request-shaping helpers it leans on are in request.go.
type Pipeline struct {
	providers providerservice.Service
	security  securityservice.Service
	audit     auditservice.Service
	upstream  *upstreamClient

	// maxRequestCostMicroCents refuses any single call costing more than
	// this, whatever budget is behind it. Zero disables it. See
	// config.MaxRequestCostMicroCents for why it exists.
	maxRequestCostMicroCents int64
}

func NewPipeline(providers providerservice.Service, sec securityservice.Service, aud auditservice.Service, maxRequestCostMicroCents int64) *Pipeline {
	return &Pipeline{
		providers:                providers,
		security:                 sec,
		audit:                    aud,
		upstream:                 newUpstreamClient(),
		maxRequestCostMicroCents: maxRequestCostMicroCents,
	}
}

func (p *Pipeline) RegisterRoutes(r chi.Router) {
	r.Get("/v1/models", p.handleModels)
	r.Post("/v1/chat/completions", p.handleChatCompletions)
	r.Post("/v1/messages", p.handleMessages)
	r.Post("/v1/embeddings", p.handleEmbeddings)
}
