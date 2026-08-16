package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/audit/service"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

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

func (h *Handler) listCallLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.ListCallLogs(r.Context(), 500)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func (h *Handler) listMyCallLogs(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	logs, err := h.service.ListMyCallLogs(r.Context(), principal.MemberID, 200)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func (h *Handler) listOperationLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.ListOperationLogs(r.Context(), 500)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}
