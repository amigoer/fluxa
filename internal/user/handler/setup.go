package handler

import (
	"errors"
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/user/repo"
)

func (h *Handler) setupStatus(w http.ResponseWriter, r *http.Request) {
	_, err := h.repo.GetOrganization(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]bool{"needsSetup": errors.Is(err, repo.ErrNotFound)})
}

type setupRequest struct {
	OrgName    string `json:"orgName"`
	AdminName  string `json:"adminName"`
	AdminEmail string `json:"adminEmail"`
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	if _, err := h.repo.GetOrganization(r.Context()); !errors.Is(err, repo.ErrNotFound) {
		httpx.Error(w, http.StatusConflict, i18n.KeyValidationFailed, "organization already set up")
		return
	}

	var req setupRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	member, err := h.service.Bootstrap(r.Context(), req.OrgName, req.AdminName, req.AdminEmail)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	if err := h.sessions.Login(r.Context(), w, member.ID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, member)
}
