package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/service"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) createQuotaRequest(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var q types.QuotaRequest
	if !decodeJSON(w, r, &q) {
		return
	}
	q.RequestedByMemberID = principal.MemberID
	created, err := h.service.RequestQuota(r.Context(), q)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) listMyQuotaRequests(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	requests, err := h.service.ListMyQuotaRequests(r.Context(), principal.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, requests)
}

func (h *Handler) listPendingQuotaRequests(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())

	if principal.Has(rbac.PermissionQuotaApproveAny) {
		requests, err := h.service.ListAllPendingQuotaRequests(r.Context())
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, requests)
		return
	}

	if !principal.Has(rbac.PermissionOrgApproveDepartmentQuota) || principal.DepartmentID == "" {
		httpx.JSON(w, http.StatusOK, []types.QuotaRequest{})
		return
	}
	requests, err := h.service.ListPendingQuotaRequestsForDepartment(r.Context(), principal.DepartmentID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, requests)
}

type decideQuotaRequestBody struct {
	Approve bool `json:"approve"`
}

func (h *Handler) decideQuotaRequest(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	id := chi.URLParam(r, "id")

	var body decideQuotaRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}

	err := h.service.DecideQuotaRequest(r.Context(), id, principal.MemberID, body.Approve, principal.Has(rbac.PermissionQuotaApproveAny))
	if errors.Is(err, service.ErrCannotApprove) {
		httpx.Error(w, http.StatusForbidden, i18n.KeyPermissionDenied, "")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
