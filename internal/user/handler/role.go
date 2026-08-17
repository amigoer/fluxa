package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	roles, err := h.service.ListRoles(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, roles)
}

type createRoleRequest struct {
	Name        string            `json:"name"`
	Permissions []rbac.Permission `json:"permissions"`
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var req createRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	role, err := h.service.CreateCustomRole(r.Context(), principal.OrgID, req.Name, req.Permissions)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, role)
}

func (h *Handler) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	codes, err := h.service.RolePermissions(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, codes)
}

type setRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

func (h *Handler) putRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setRolePermissionsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetRolePermissions(r.Context(), id, req.Permissions); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
