package service

import (
	"context"
	"fmt"

	"github.com/amigoer/fluxa/internal/user/types"
)

// BootstrapService is the deployment's first-run path: it exists to be
// called exactly once, and every other service assumes it already has.
type BootstrapService interface {
	Bootstrap(ctx context.Context, orgName, adminName, adminEmail string) (types.Member, error)
}

// Bootstrap creates the single organization this deployment serves, its
// built-in roles, and its first member as a super admin. It is meant to
// run exactly once, from the first-run setup flow (see the handler
// package), guarded by GetOrganization returning repo.ErrNotFound.
func (s *service) Bootstrap(ctx context.Context, orgName, adminName, adminEmail string) (types.Member, error) {
	org, err := s.repo.CreateOrganization(ctx, orgName)
	if err != nil {
		return types.Member{}, fmt.Errorf("user: create organization: %w", err)
	}

	roles, err := s.EnsureBuiltinRoles(ctx, org.ID)
	if err != nil {
		return types.Member{}, err
	}

	member, err := s.repo.CreateMember(ctx, types.Member{
		OrgID:  org.ID,
		RoleID: roles[types.RoleSuperAdmin].ID,
		Name:   adminName,
		Email:  &adminEmail,
		Status: types.MemberStatusActive,
	})
	if err != nil {
		return types.Member{}, fmt.Errorf("user: create first admin: %w", err)
	}

	return member, nil
}
