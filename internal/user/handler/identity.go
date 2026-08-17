package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

func (h *Handler) getIdentityConfig(w http.ResponseWriter, r *http.Request) {
	provider := types.IdentityProvider(chi.URLParam(r, "provider"))
	cfg, err := h.service.GetIdentityConfig(r.Context(), provider)
	if errors.Is(err, repo.ErrNotFound) {
		httpx.JSON(w, http.StatusOK, types.IdentityConfig{Provider: provider})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	cfg.AppSecret = maskSecret(cfg.AppSecret)
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) putIdentityConfig(w http.ResponseWriter, r *http.Request) {
	provider := types.IdentityProvider(chi.URLParam(r, "provider"))
	var cfg types.IdentityConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}
	cfg.Provider = provider
	if err := h.service.UpsertIdentityConfig(r.Context(), cfg); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// maskSecret keeps enough of a stored credential for an admin to
// recognise which one is saved, without shipping the secret itself to
// the browser.
func maskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:4] + "****"
}
