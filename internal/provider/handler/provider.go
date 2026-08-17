package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	providers, err := h.service.ListProviders(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, providers)
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var p types.Provider
	if !decodeJSON(w, r, &p) {
		return
	}
	p.OrgID = principal.OrgID
	created, err := h.service.CreateProvider(r.Context(), p)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}
