// Identity provider configuration and the auth settings that decide which
// login methods a deployment offers.

package handler

import (
	"errors"
	"net/http"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

// -- Identity configs & auth settings -------------------------------------

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

func (h *Handler) getAuthSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetAuthSettings(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

func (h *Handler) putAuthSettings(w http.ResponseWriter, r *http.Request) {
	var settings types.AuthSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	if err := h.service.UpdateAuthSettings(r.Context(), settings); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
