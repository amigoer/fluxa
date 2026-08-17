package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	member, err := h.repo.GetMember(r.Context(), principal.MemberID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	role, err := h.repo.GetRoleByID(r.Context(), member.RoleID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	departmentName := ""
	if member.DepartmentID != nil {
		if dept, err := h.repo.GetDepartment(r.Context(), *member.DepartmentID); err == nil {
			departmentName = dept.Name
		}
	}

	// The org name is display-only, but every authenticated screen shows
	// it (the sidebar brand block), so it rides along with /api/me rather
	// than costing a second round trip on every page load.
	orgName := ""
	if org, err := h.repo.GetOrganization(r.Context()); err == nil {
		orgName = org.Name
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"member":         member,
		"permissions":    principal.Permissions,
		"roleName":       role.Name,
		"departmentName": departmentName,
		"orgName":        orgName,
	})
}
