package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listProviderHealth(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	providers, err := h.service.ListProviders(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	out := make([]types.ProviderHealth, 0, len(providers))
	for _, p := range providers {
		health, err := h.repo.GetProviderHealth(r.Context(), p.ID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		out = append(out, health)
	}
	httpx.JSON(w, http.StatusOK, out)
}
