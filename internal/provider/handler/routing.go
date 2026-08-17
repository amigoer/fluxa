package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

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
