package handler

import (
	"encoding/json"
	"net/http"

	"github.com/amigoer/fluxa/internal/security/service"
	"github.com/amigoer/fluxa/internal/security/types"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/rbac"
)

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

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRules(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var rule types.DLPRule
	if !decodeJSON(w, r, &rule) {
		return
	}
	created, err := h.service.CreateRule(r.Context(), rule)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

type setEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setRuleEnabled(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setEnabledRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetRuleEnabled(r.Context(), id, req.Enabled); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteRule(r.Context(), id); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context(), 200)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, events)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return false
	}
	return true
}
