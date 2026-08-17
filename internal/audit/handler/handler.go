package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/audit/service"
	"github.com/amigoer/fluxa/internal/rbac"
)

// Handler wires the Audit module's HTTP surface. The route table lives
// here and nothing else does: each endpoint's implementation sits in the
// file for its feature, next to this one.
type Handler struct {
	service service.Service
}

func New(service service.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(rbac.Require(rbac.PermissionAuditViewCallLogs)).Get("/api/call-logs", h.listCallLogs)
	r.Get("/api/call-logs/mine", h.listMyCallLogs)
	r.With(rbac.Require(rbac.PermissionAuditViewOperationLogs)).Get("/api/operation-logs", h.listOperationLogs)
}
