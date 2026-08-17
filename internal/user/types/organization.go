// Package types holds the User module's domain entities. The module
// implements DESIGN.md 7.1: organization/department/member structure,
// RBAC roles, identity sources, the local-account fallback, and the
// server-side session that keeps a browser logged in.
//
// It imports nothing else from this module, so user/repo, user/service,
// user/session and user/handler can all depend on it without an import
// cycle. One entity per file, named to match the repo, service and
// handler file that works with it.
package types

import "time"

type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
}
