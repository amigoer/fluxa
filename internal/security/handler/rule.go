package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/security/types"
)

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
