package provider

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/quota"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

// Handler wires the Provider module's HTTP surface: providers/models,
// procurement, routing rules, virtual keys, department quota pools, and
// quota requests. Like the User module, admin and employee views share
// these endpoints; per-route rbac.Require (or an ownership check inside
// the handler for things like "my own virtual keys") is what limits what
// each caller can see or do.
type Handler struct {
	service *Service
	repo    *Repo
}

func NewHandler(service *Service, repo *Repo) *Handler {
	return &Handler{service: service, repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(rbac.Require(rbac.PermissionProviderView)).Get("/api/providers", h.listProviders)
	r.With(rbac.Require(rbac.PermissionProviderManageCredentials)).Post("/api/providers", h.createProvider)

	r.With(rbac.Require(rbac.PermissionProviderView)).Get("/api/models", h.listModels)
	r.Get("/api/models/published", h.listPublishedModels) // every authenticated caller needs this for pricing/routing/playground pickers
	r.With(rbac.Require(rbac.PermissionProviderManageCredentials)).Post("/api/models", h.createModel)

	r.With(rbac.Require(rbac.PermissionProviderView)).Get("/api/procurement", h.listProcurement)
	r.With(rbac.Require(rbac.PermissionProviderRecordProcurement)).Post("/api/procurement", h.recordProcurement)

	r.With(rbac.Require(rbac.PermissionProviderManageRouting)).Get("/api/routing/global", h.listGlobalRouting)
	r.With(rbac.Require(rbac.PermissionProviderManageRouting)).Post("/api/routing/global", h.createGlobalRoutingRule)
	r.With(rbac.Require(rbac.PermissionOrgManagePersonalRouting)).Get("/api/routing/mine", h.listMyRouting)
	r.With(rbac.Require(rbac.PermissionOrgManagePersonalRouting)).Post("/api/routing/mine", h.createMyRoutingRule)

	r.Get("/api/virtual-keys", h.listVirtualKeys) // scoped to self unless the caller has org.manage_keys
	r.With(rbac.Require(rbac.PermissionOrgManageKeys)).Post("/api/virtual-keys", h.createVirtualKey)
	r.Post("/api/virtual-keys/{id}/revoke", h.revokeVirtualKey) // self-owned or org.manage_keys, checked inline

	r.With(rbac.Require(rbac.PermissionOrgApproveDepartmentQuota)).Get("/api/department-quota-pools/{departmentId}", h.getDepartmentQuotaPool)
	r.With(rbac.Require(rbac.PermissionOrgManageDepartments)).Put("/api/department-quota-pools/{departmentId}", h.setDepartmentQuotaPool)

	r.With(rbac.Require(rbac.PermissionOrgRequestQuota)).Post("/api/quota-requests", h.createQuotaRequest)
	r.With(rbac.Require(rbac.PermissionOrgRequestQuota)).Get("/api/quota-requests/mine", h.listMyQuotaRequests)
	r.Get("/api/quota-requests/pending", h.listPendingQuotaRequests) // department-lead or admin, scoped inline
	r.Post("/api/quota-requests/{id}/decide", h.decideQuotaRequest)

	r.With(rbac.Require(rbac.PermissionProviderView)).Get("/api/provider-health", h.listProviderHealth)
}

// -- Providers & models -----------------------------------------------

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	providers, err := h.service.ListProviders(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, providers)
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var p types.Provider
	if !decodeJSON(w, r, &p) {
		return
	}
	p.OrgID = principal.OrgID
	created, err := h.service.CreateProvider(r.Context(), p)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	models, err := h.service.ListModels(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, models)
}

func (h *Handler) listPublishedModels(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	models, err := h.service.ListPublishedModels(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, models)
}

func (h *Handler) createModel(w http.ResponseWriter, r *http.Request) {
	var m types.Model
	if !decodeJSON(w, r, &m) {
		return
	}
	created, err := h.service.CreateModel(r.Context(), m)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

// -- Procurement ------------------------------------------------------

func (h *Handler) listProcurement(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	records, err := h.service.ListProcurementRecords(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, records)
}

func (h *Handler) recordProcurement(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var rec types.ProcurementRecord
	if !decodeJSON(w, r, &rec) {
		return
	}
	rec.RecordedByMemberID = principal.MemberID
	created, err := h.service.RecordProcurement(r.Context(), rec)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

// -- Routing ------------------------------------------------------------

func (h *Handler) listGlobalRouting(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRoutingChain(r.Context(), types.RoutingScopeGlobal, nil)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) createGlobalRoutingRule(w http.ResponseWriter, r *http.Request) {
	var rule types.RoutingRule
	if !decodeJSON(w, r, &rule) {
		return
	}
	rule.Scope = types.RoutingScopeGlobal
	rule.OwnerMemberID = nil
	created, err := h.service.CreateRoutingRule(r.Context(), rule)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) listMyRouting(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	rules, err := h.service.ListRoutingChain(r.Context(), types.RoutingScopePersonal, &principal.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) createMyRoutingRule(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var rule types.RoutingRule
	if !decodeJSON(w, r, &rule) {
		return
	}
	rule.Scope = types.RoutingScopePersonal
	rule.OwnerMemberID = &principal.MemberID
	created, err := h.service.CreateRoutingRule(r.Context(), rule)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

// -- Virtual keys -----------------------------------------------------

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

	if err := h.service.RevokeVirtualKey(r.Context(), id); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Department quota pools ----------------------------------------------

func (h *Handler) getDepartmentQuotaPool(w http.ResponseWriter, r *http.Request) {
	departmentID := chi.URLParam(r, "departmentId")
	balance, err := h.service.DepartmentQuotaBalance(r.Context(), departmentID)
	if errors.Is(err, ErrNotFound) {
		// A department with no pool yet is an empty pool, not an error --
		// but it has to serialize like every other balance, so the client
		// isn't parsing two different shapes off one endpoint.
		httpx.JSON(w, http.StatusOK, quota.Balance{})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, balance)
}

type setQuotaPoolRequest struct {
	TotalCents int64 `json:"totalCents"`
}

func (h *Handler) setDepartmentQuotaPool(w http.ResponseWriter, r *http.Request) {
	departmentID := chi.URLParam(r, "departmentId")
	var req setQuotaPoolRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetDepartmentQuotaPool(r.Context(), departmentID, req.TotalCents); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Quota requests -----------------------------------------------------

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
	if errors.Is(err, ErrCannotApprove) {
		httpx.Error(w, http.StatusForbidden, i18n.KeyPermissionDenied, "")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// -- Provider health ------------------------------------------------------

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

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return false
	}
	return true
}
