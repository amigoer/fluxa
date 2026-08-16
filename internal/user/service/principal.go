package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/rbac"
)

// PrincipalService turns an authenticated member into the rbac.Principal
// every downstream permission check reads. The session middleware calls
// it on each request.
type PrincipalService interface {
	LoadPrincipal(ctx context.Context, memberID string) (rbac.Principal, error)
}

// LoadPrincipal builds the rbac.Principal for an already-authenticated
// member, loading their role's permission grants from the database.
func (s *service) LoadPrincipal(ctx context.Context, memberID string) (rbac.Principal, error) {
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
