package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// MemberService owns the people in the org: who is in it, and where each
// one sits in the department/role structure.
type MemberService interface {
	ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error)
	GetMember(ctx context.Context, id string) (types.Member, error)
	ApproveMember(ctx context.Context, id string) error
	UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error
	UpdateMemberRole(ctx context.Context, id, roleID string) error
	UpdateMemberContact(ctx context.Context, id string, email, phone *string) error
}

func (s *service) ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error) {
	return s.repo.ListMembers(ctx, orgID, departmentID)
}

func (s *service) GetMember(ctx context.Context, id string) (types.Member, error) {
	return s.repo.GetMember(ctx, id)
}

// ApproveMember moves a pending local-account registration to active
// (DESIGN.md 8.3).
func (s *service) ApproveMember(ctx context.Context, id string) error {
	return s.repo.UpdateMemberStatus(ctx, id, types.MemberStatusActive)
}

func (s *service) UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error {
	return s.repo.UpdateMemberDepartment(ctx, id, departmentID)
}

func (s *service) UpdateMemberRole(ctx context.Context, id, roleID string) error {
	return s.repo.UpdateMemberRole(ctx, id, roleID)
}

// UpdateMemberContact repairs the address or number an identity source
// never supplied. It is the way out for a member who would otherwise be
// unreachable by every enabled login method (DESIGN.md 7.1: an identity
// source can be switched off, and the people it created stay).
func (s *service) UpdateMemberContact(ctx context.Context, id string, email, phone *string) error {
	return s.repo.UpdateMemberContact(ctx, id, email, phone)
}
