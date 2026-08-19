package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/repo"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listVirtualKeys(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())

	if principal.Has(rbac.PermissionOrgManageKeys) {
		keys, err := h.service.ListVirtualKeys(r.Context(), principal.OrgID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, keys)
		return
	}

	keys, err := h.repo.ListActiveVirtualKeysByMember(r.Context(), principal.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, keys)
}

func (h *Handler) createVirtualKey(w http.ResponseWriter, r *http.Request) {
	var k types.VirtualKey
	if !decodeJSON(w, r, &k) {
		return
	}
	created, raw, err := h.service.CreateVirtualKey(r.Context(), k)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"key": created, "secret": raw})
}

func (h *Handler) revokeVirtualKey(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !principal.Has(rbac.PermissionOrgManageKeys) {
		owned, err := h.repo.ListActiveVirtualKeysByMember(r.Context(), principal.MemberID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		isOwner := false
		for _, k := range owned {
			if k.ID == id {
				isOwner = true
				break
			}
		}
		if !isOwner {
			httpx.Error(w, http.StatusForbidden, i18n.KeyPermissionDenied, "")
			return
		}
	}

	if err := h.service.RevokeVirtualKey(r.Context(), id); errors.Is(err, repo.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, i18n.KeyNotFound, "")
		return
	} else if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
