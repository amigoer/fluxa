package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/amigoer/fluxa/internal/rbac"
	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

// RoleService owns what a role is allowed to do: the fixed built-in
// four, and any custom role an org adds beside them.
type RoleService interface {
	EnsureBuiltinRoles(ctx context.Context, orgID string) (map[string]types.Role, error)
	ListRoles(ctx context.Context, orgID string) ([]types.Role, error)
	RolePermissions(ctx context.Context, roleID string) ([]string, error)
	SetRolePermissions(ctx context.Context, roleID string, codes []string) error
	CreateCustomRole(ctx context.Context, orgID, name string, perms []rbac.Permission) (types.Role, error)
}

// allPermissions lists every permission point that exists, used to grant
// the super admin role everything without hand-maintaining a duplicate
// list that can drift from internal/rbac/permission.go.
var allPermissions = []rbac.Permission{
	rbac.PermissionProviderManageCredentials,
	rbac.PermissionProviderView,
	rbac.PermissionProviderManageRouting,
	rbac.PermissionProviderRecordProcurement,
	rbac.PermissionProviderUsePlayground,
	rbac.PermissionOrgManageMembers,
	rbac.PermissionOrgManageDepartments,
	rbac.PermissionOrgManageRoles,
	rbac.PermissionOrgManageIdentitySources,
	rbac.PermissionOrgManageNotifyChannels,
	rbac.PermissionOrgManageKeys,
	rbac.PermissionOrgViewOwnUsage,
	rbac.PermissionOrgManagePersonalRouting,
	rbac.PermissionOrgRequestQuota,
	rbac.PermissionOrgApproveDepartmentQuota,
	rbac.PermissionQuotaAdjustAnyMember,
	rbac.PermissionQuotaApproveAny,
	rbac.PermissionSecurityManageDLPRules,
	rbac.PermissionSecurityViewEvents,
	rbac.PermissionAuditViewCallLogs,
	rbac.PermissionAuditViewOperationLogs,
}

// builtinRolePermissions defines what each built-in role can do. Admin
// gets everything a super admin gets except managing roles themselves --
// only a super admin can redefine what roles are allowed to do, so an
// admin can never grant themselves a permission they don't already have.
func builtinRolePermissions() map[string][]rbac.Permission {
	adminPermissions := make([]rbac.Permission, 0, len(allPermissions))
	for _, p := range allPermissions {
		if p != rbac.PermissionOrgManageRoles {
			adminPermissions = append(adminPermissions, p)
		}
	}

	return map[string][]rbac.Permission{
		types.RoleSuperAdmin: allPermissions,
		types.RoleAdmin:      adminPermissions,
		types.RoleDepartmentLead: {
			rbac.PermissionOrgViewOwnUsage,
			rbac.PermissionOrgManagePersonalRouting,
			rbac.PermissionOrgRequestQuota,
			rbac.PermissionOrgApproveDepartmentQuota,
		},
		types.RoleEmployee: {
			rbac.PermissionOrgViewOwnUsage,
			rbac.PermissionOrgManagePersonalRouting,
			rbac.PermissionOrgRequestQuota,
		},
	}
}

// EnsureBuiltinRoles creates the four built-in roles for orgID if they
// don't already exist, granting each its fixed permission set. It is
// idempotent so it is safe to call defensively.
func (s *service) EnsureBuiltinRoles(ctx context.Context, orgID string) (map[string]types.Role, error) {
	roles := make(map[string]types.Role, 4)

	for name, perms := range builtinRolePermissions() {
		role, err := s.repo.GetRoleByName(ctx, orgID, name)
		if errors.Is(err, repo.ErrNotFound) {
			role, err = s.repo.CreateRole(ctx, orgID, name, true)
		}
		if err != nil {
			return nil, fmt.Errorf("user: ensure role %q: %w", name, err)
		}

		codes := make([]string, len(perms))
		for i, p := range perms {
			codes[i] = string(p)
		}
		if err := s.repo.GrantPermissions(ctx, role.ID, codes); err != nil {
			return nil, fmt.Errorf("user: grant permissions for %q: %w", name, err)
		}

		roles[name] = role
	}

	return roles, nil
}

func (s *service) ListRoles(ctx context.Context, orgID string) ([]types.Role, error) {
	return s.repo.ListRoles(ctx, orgID)
}

// RolePermissions returns the permission codes granted to roleID, for
// the role-permissions admin page to render as a checkbox grid.
func (s *service) RolePermissions(ctx context.Context, roleID string) ([]string, error) {
	return s.repo.RolePermissionCodes(ctx, roleID)
}

// SetRolePermissions replaces roleID's permission grants outright. Used
// both for custom roles and for editing a built-in role's grants -- the
// route is gated by org.manage_roles either way, which only a super
// admin holds (see builtinRolePermissions), so this can't be used to
// grant oneself something one didn't already have.
func (s *service) SetRolePermissions(ctx context.Context, roleID string, codes []string) error {
	return s.repo.GrantPermissions(ctx, roleID, codes)
}

// CreateCustomRole adds an org-specific role beyond the four built-ins
// (DESIGN.md 7.1: e.g. carving out a "finance" or "security" role).
func (s *service) CreateCustomRole(ctx context.Context, orgID, name string, perms []rbac.Permission) (types.Role, error) {
	role, err := s.repo.CreateRole(ctx, orgID, name, false)
	if err != nil {
		return types.Role{}, err
	}
	codes := make([]string, len(perms))
	for i, p := range perms {
		codes[i] = string(p)
	}
	if err := s.repo.GrantPermissions(ctx, role.ID, codes); err != nil {
		return types.Role{}, err
	}
	return role, nil
}
