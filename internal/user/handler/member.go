package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var departmentID *string
	if v := r.URL.Query().Get("departmentId"); v != "" {
		departmentID = &v
	}
	members, err := h.service.ListMembers(r.Context(), principal.OrgID, departmentID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, members)
}

func (h *Handler) approveMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.ApproveMember(r.Context(), id); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type updateDepartmentRequest struct {
	DepartmentID *string `json:"departmentId"`
}

func (h *Handler) updateMemberDepartment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateDepartmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.UpdateMemberDepartment(r.Context(), id, req.DepartmentID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

type updateRoleRequest struct {
	RoleID string `json:"roleId"`
}

func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.UpdateMemberRole(r.Context(), id, req.RoleID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
