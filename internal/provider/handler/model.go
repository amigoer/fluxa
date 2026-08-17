package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	models, err := h.service.ListModels(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, models)
}

func (h *Handler) listPublishedModels(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	models, err := h.service.ListPublishedModels(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, models)
}

func (h *Handler) createModel(w http.ResponseWriter, r *http.Request) {
	var m types.Model
	if !decodeJSON(w, r, &m) {
		return
	}
	created, err := h.service.CreateModel(r.Context(), m)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}
