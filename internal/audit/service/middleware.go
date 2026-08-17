package service

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/amigoer/fluxa/internal/rbac"
)

// actions names each mutating management route, keyed by "METHOD <chi
// route pattern>". The code is what the 操作审计 page groups and filters
// by, so it has to stay stable even if the URL is reshaped -- that is the
// whole reason for spelling the mapping out here instead of deriving it
// from the path.
//
// A route missing from this map is still recorded, under its own
// "METHOD /pattern" string. Falling back rather than dropping is
// deliberate: an audit trail whose completeness depends on somebody
// remembering to add a line is exactly the trail that turned out to be
// empty.
var actions = map[string]string{
	// 组织与权限
	"POST /api/members/{id}/approve":        "member.approve",
	"PATCH /api/members/{id}/department":    "member.department.update",
	"PATCH /api/members/{id}/role":          "member.role.update",
	"PATCH /api/members/{id}/contact":       "member.contact.update",
	"POST /api/departments":                 "department.create",
	"PATCH /api/departments/{id}/lead":      "department.lead.update",
	"POST /api/roles":                       "role.create",
	"PUT /api/roles/{id}/permissions":       "role.permissions.update",
	"PUT /api/identity-configs/{provider}":  "identity_config.update",
	"PUT /api/auth-settings":                "auth_settings.update",
	"PUT /api/notify-channels/{kind}":       "notify_channel.update",
	"POST /api/notify-channels/{kind}/test": "notify_channel.test",

	// 资源管理
	"POST /api/providers":                            "provider.create",
	"POST /api/models":                               "model.create",
	"POST /api/procurement":                          "procurement.record",
	"POST /api/routing/global":                       "routing.global.create",
	"POST /api/routing/mine":                         "routing.personal.create",
	"POST /api/virtual-keys":                         "virtual_key.create",
	"POST /api/virtual-keys/{id}/revoke":             "virtual_key.revoke",
	"PUT /api/department-quota-pools/{departmentId}": "department_quota.update",

	// 配额
	"POST /api/quota-requests":             "quota_request.create",
	"POST /api/quota-requests/{id}/decide": "quota_request.decide",

	// 安全
	"POST /api/dlp-rules":               "dlp_rule.create",
	"PATCH /api/dlp-rules/{id}/enabled": "dlp_rule.enabled.update",
	"DELETE /api/dlp-rules/{id}":        "dlp_rule.delete",
}

// skip lists mutating routes that are not management actions. Logging
// out is a session event, not something an admin did to the deployment,
// and letting it through would bury the real entries.
var skip = map[string]bool{
	"POST /api/auth/logout": true,
}

// MutationRecorder is the middleware half of the operation audit trail:
// it is what actually fills the log OperationLogService reads back.
type MutationRecorder interface {
	RecordMutations(next http.Handler) http.Handler
}

// RecordMutations writes an operation audit entry for every successful
// state-changing request on the session-authenticated management API.
//
// It is middleware rather than a LogOperation call inside each handler
// because the per-handler version was never actually written: the trail
// had exactly one caller (a quota branch in the gateway), so every admin
// action -- role changes, provider credentials, DLP rules, key issuance
// -- went unrecorded while the page promised "所有管理动作的不可变记录".
// Sitting in the middleware chain, a new route is covered the moment it
// is mounted.
//
// Only the actor, the action and the request path are stored. Request
// bodies are deliberately never touched: they carry OAuth app secrets,
// SMTP passwords and freshly minted virtual keys, none of which belong in
// a table the whole admin console can read.
func (s *service) RecordMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mutates(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		rec := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(rec, r)

		// A handler that refused the request changed nothing, so there is
		// nothing to record. Status 0 means the handler wrote no header,
		// which net/http turns into a 200.
		if status := rec.Status(); status != 0 && (status < 200 || status > 299) {
			return
		}

		route := routeKey(r)
		if skip[route] {
			return
		}

		// Only a session-authenticated caller reaches this middleware, but
		// guard anyway rather than attributing an action to nobody.
		principal, ok := rbac.FromContext(r.Context())
		if !ok {
			return
		}

		action, found := actions[route]
		if !found {
			action = route
		}

		// The response is already on the wire, so a failed audit write can
		// no longer change the outcome -- and it must not be able to.
		// WithoutCancel keeps the write alive even when the caller hung up
		// the moment their request succeeded.
		_ = s.LogOperation(context.WithoutCancel(r.Context()), principal.MemberID, action, r.URL.Path)
	})
}

func mutates(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// routeKey is the request's "METHOD <pattern>" identity. The pattern is
// read after the handler has run, by which point chi has resolved it.
func routeKey(r *http.Request) string {
	pattern := ""
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		pattern = rctx.RoutePattern()
	}
	if pattern == "" {
		pattern = r.URL.Path
	}
	return r.Method + " " + pattern
}
