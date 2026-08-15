package user

import (
	"context"
	"fmt"

	"github.com/amigoer/fluxa/internal/rbac"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
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
		RoleSuperAdmin: allPermissions,
		RoleAdmin:      adminPermissions,
		RoleDepartmentLead: {
			rbac.PermissionOrgViewOwnUsage,
			rbac.PermissionOrgManagePersonalRouting,
			rbac.PermissionOrgRequestQuota,
			rbac.PermissionOrgApproveDepartmentQuota,
		},
		RoleEmployee: {
			rbac.PermissionOrgViewOwnUsage,
			rbac.PermissionOrgManagePersonalRouting,
			rbac.PermissionOrgRequestQuota,
		},
	}
}

// EnsureBuiltinRoles creates the four built-in roles for orgID if they
// don't already exist, granting each its fixed permission set. It is
// idempotent so it is safe to call defensively.
func (s *Service) EnsureBuiltinRoles(ctx context.Context, orgID string) (map[string]Role, error) {
	roles := make(map[string]Role, 4)

	for name, perms := range builtinRolePermissions() {
		role, err := s.repo.GetRoleByName(ctx, orgID, name)
		if err == ErrNotFound {
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

// Bootstrap creates the single organization this deployment serves, its
// built-in roles, and its first member as a super admin. It is meant to
// run exactly once, from the first-run setup flow (see handler.go),
// guarded by GetOrganization returning ErrNotFound.
func (s *Service) Bootstrap(ctx context.Context, orgName, adminName, adminEmail string) (Member, error) {
	org, err := s.repo.CreateOrganization(ctx, orgName)
	if err != nil {
		return Member{}, fmt.Errorf("user: create organization: %w", err)
	}

	roles, err := s.EnsureBuiltinRoles(ctx, org.ID)
	if err != nil {
		return Member{}, err
	}

	member, err := s.repo.CreateMember(ctx, Member{
		OrgID:  org.ID,
		RoleID: roles[RoleSuperAdmin].ID,
		Name:   adminName,
		Email:  &adminEmail,
		Status: MemberStatusActive,
	})
	if err != nil {
		return Member{}, fmt.Errorf("user: create first admin: %w", err)
	}

	return member, nil
}

// LoadPrincipal builds the rbac.Principal for an already-authenticated
// member, loading their role's permission grants from the database.
func (s *Service) LoadPrincipal(ctx context.Context, memberID string) (rbac.Principal, error) {
	member, err := s.repo.GetMember(ctx, memberID)
	if err != nil {
		return rbac.Principal{}, err
	}

	codes, err := s.repo.RolePermissionCodes(ctx, member.RoleID)
	if err != nil {
		return rbac.Principal{}, err
	}

	permissions := make(map[rbac.Permission]struct{}, len(codes))
	for _, c := range codes {
		permissions[rbac.Permission(c)] = struct{}{}
	}

	departmentID := ""
	if member.DepartmentID != nil {
		departmentID = *member.DepartmentID
	}

	return rbac.Principal{
		MemberID:     member.ID,
		OrgID:        member.OrgID,
		DepartmentID: departmentID,
		Permissions:  permissions,
	}, nil
}

// IsDepartmentLead reports whether memberID leads departmentID, used by
// the provider module's quota approval routing (DESIGN.md 8.4: a
// department's own lead approves its quota requests by default).
func (s *Service) IsDepartmentLead(ctx context.Context, memberID, departmentID string) (bool, error) {
	dept, err := s.repo.GetDepartment(ctx, departmentID)
	if err != nil {
		return false, err
	}
	return dept.LeadMemberID != nil && *dept.LeadMemberID == memberID, nil
}

func (s *Service) CreateDepartment(ctx context.Context, orgID, name string) (Department, error) {
	return s.repo.CreateDepartment(ctx, orgID, name)
}

func (s *Service) ListDepartments(ctx context.Context, orgID string) ([]Department, error) {
	return s.repo.ListDepartments(ctx, orgID)
}

func (s *Service) SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error {
	return s.repo.SetDepartmentLead(ctx, departmentID, leadMemberID)
}

func (s *Service) ListMembers(ctx context.Context, orgID string, departmentID *string) ([]Member, error) {
	return s.repo.ListMembers(ctx, orgID, departmentID)
}

func (s *Service) GetMember(ctx context.Context, id string) (Member, error) {
	return s.repo.GetMember(ctx, id)
}

// ApproveMember moves a pending local-account registration to active
// (DESIGN.md 8.3).
func (s *Service) ApproveMember(ctx context.Context, id string) error {
	return s.repo.UpdateMemberStatus(ctx, id, MemberStatusActive)
}

func (s *Service) UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error {
	return s.repo.UpdateMemberDepartment(ctx, id, departmentID)
}

func (s *Service) UpdateMemberRole(ctx context.Context, id, roleID string) error {
	return s.repo.UpdateMemberRole(ctx, id, roleID)
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]Role, error) {
	return s.repo.ListRoles(ctx, orgID)
}

// CreateCustomRole adds an org-specific role beyond the four built-ins
// (DESIGN.md 7.1: e.g. carving out a "finance" or "security" role).
func (s *Service) CreateCustomRole(ctx context.Context, orgID, name string, perms []rbac.Permission) (Role, error) {
	role, err := s.repo.CreateRole(ctx, orgID, name, false)
	if err != nil {
		return Role{}, err
	}
	codes := make([]string, len(perms))
	for i, p := range perms {
		codes[i] = string(p)
	}
	if err := s.repo.GrantPermissions(ctx, role.ID, codes); err != nil {
		return Role{}, err
	}
	return role, nil
}

func (s *Service) GetIdentityConfig(ctx context.Context, provider IdentityProvider) (IdentityConfig, error) {
	return s.repo.GetIdentityConfig(ctx, provider)
}

func (s *Service) UpsertIdentityConfig(ctx context.Context, c IdentityConfig) error {
	return s.repo.UpsertIdentityConfig(ctx, c)
}

func (s *Service) GetAuthSettings(ctx context.Context) (AuthSettings, error) {
	return s.repo.GetAuthSettings(ctx)
}

func (s *Service) UpdateAuthSettings(ctx context.Context, settings AuthSettings) error {
	return s.repo.UpdateAuthSettings(ctx, settings)
}

func (s *Service) GetNotifyChannel(ctx context.Context, kind NotifyChannelKind) (NotifyChannel, error) {
	return s.repo.GetNotifyChannel(ctx, kind)
}

func (s *Service) UpsertNotifyChannel(ctx context.Context, c NotifyChannel) error {
	return s.repo.UpsertNotifyChannel(ctx, c)
}
