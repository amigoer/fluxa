package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/security/service"
)

// Handler wires the Security module's HTTP surface: DLP rule
// administration and the security event trail. The route table lives
// here and nothing else does: each endpoint's implementation sits in the
// file for its feature, next to this one.
type Handler struct {
	service service.Service
}

func New(service service.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(rbac.Require(rbac.PermissionSecurityManageDLPRules)).Get("/api/dlp-rules", h.listRules)
	r.With(rbac.Require(rbac.PermissionSecurityManageDLPRules)).Post("/api/dlp-rules", h.createRule)
	r.With(rbac.Require(rbac.PermissionSecurityManageDLPRules)).Patch("/api/dlp-rules/{id}/enabled", h.setRuleEnabled)
	r.With(rbac.Require(rbac.PermissionSecurityManageDLPRules)).Delete("/api/dlp-rules/{id}", h.deleteRule)

	r.With(rbac.Require(rbac.PermissionSecurityViewEvents)).Get("/api/security-events", h.listEvents)
}
