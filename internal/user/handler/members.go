// Member, department and role administration.

package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/go-chi/chi/v5"
)

// -- Members ----------------------------------------------------------------

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

// -- Departments --------------------------------------------------------

func (h *Handler) listDepartments(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	departments, err := h.service.ListDepartments(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, departments)
}

type createDepartmentRequest struct {
	Name string `json:"name"`
}

func (h *Handler) createDepartment(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var req createDepartmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	dept, err := h.service.CreateDepartment(r.Context(), principal.OrgID, req.Name)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dept)
}

type setLeadRequest struct {
	MemberID *string `json:"memberId"`
}

func (h *Handler) setDepartmentLead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setLeadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetDepartmentLead(r.Context(), id, req.MemberID); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Roles --------------------------------------------------------------

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
