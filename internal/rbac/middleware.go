package rbac

import (
	"context"
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
)

// Principal is the authenticated caller a request is running as. The
// user module builds one from the session's member and role after
// validating the session token and stores it on the request context
// with WithPrincipal; everything downstream only ever reads it back
// through FromContext.
type Principal struct {
	MemberID     string
	OrgID        string
	DepartmentID string // empty if the member has no department
	Permissions  map[Permission]struct{}
}

// Has reports whether the principal was granted perm through their role.
func (p Principal) Has(perm Permission) bool {
	_, ok := p.Permissions[perm]
	return ok
}

type contextKey int

const principalKey contextKey = iota

// WithPrincipal returns a context carrying p, for FromContext to read
// back later in the request lifecycle.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// FromContext returns the principal stored by WithPrincipal, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Require builds middleware that rejects the request unless the
// authenticated principal holds perm. It must run after whatever
// middleware establishes the principal (the user module's session
// middleware), since it only ever reads what is already on the context.
func Require(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := FromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, i18n.KeySessionExpired, "")
				return
			}
			if !principal.Has(perm) {
				httpx.Error(w, http.StatusForbidden, i18n.KeyPermissionDenied, string(perm))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
