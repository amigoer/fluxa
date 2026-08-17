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
}

func NewPipeline(providers providerservice.Service, sec securityservice.Service, aud auditservice.Service) *Pipeline {
	return &Pipeline{providers: providers, security: sec, audit: aud, upstream: newUpstreamClient()}
}

func (p *Pipeline) RegisterRoutes(r chi.Router) {
	r.Post("/v1/chat/completions", p.handleChatCompletions)
}
