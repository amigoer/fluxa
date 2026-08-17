package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/provider/repo"
	"github.com/amigoer/fluxa/internal/provider/service"
	"github.com/amigoer/fluxa/internal/rbac"
)

// Handler wires the Provider module's HTTP surface: providers/models,
// procurement, routing rules, virtual keys, department quota pools, and
// quota requests. Like the User module, admin and employee views share
// these endpoints; per-route rbac.Require (or an ownership check inside
// the handler for things like "my own virtual keys") is what limits what
// each caller can see or do.
//
// The route table lives here and nothing else does: each endpoint's
// implementation sits in the file for its feature, named to match the
// repo and service file it talks to.
type Handler struct {
	service service.Service
	repo    repo.Repo
}

func New(service service.Service, repo repo.Repo) *Handler {
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
