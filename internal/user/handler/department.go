package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

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
