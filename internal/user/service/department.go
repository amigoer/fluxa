package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// DepartmentService owns the org's department tree and its leads.
type DepartmentService interface {
	CreateDepartment(ctx context.Context, orgID, name string) (types.Department, error)
	ListDepartments(ctx context.Context, orgID string) ([]types.Department, error)
	SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error
	IsDepartmentLead(ctx context.Context, memberID, departmentID string) (bool, error)
}

func (s *service) CreateDepartment(ctx context.Context, orgID, name string) (types.Department, error) {
	return s.repo.CreateDepartment(ctx, orgID, name)
}

func (s *service) ListDepartments(ctx context.Context, orgID string) ([]types.Department, error) {
	return s.repo.ListDepartments(ctx, orgID)
}

func (s *service) SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error {
	return s.repo.SetDepartmentLead(ctx, departmentID, leadMemberID)
}

// IsDepartmentLead reports whether memberID leads departmentID
// (DESIGN.md 8.4: a department's own lead approves its quota requests by
// default). Nothing calls it today: the provider module answers the same
// question with a single SQL join inside its own repo rather than
// reaching across module boundaries -- see CanApproveQuotaRequest there
// for why. This stays as the in-module way to ask.
func (s *service) IsDepartmentLead(ctx context.Context, memberID, departmentID string) (bool, error) {
	dept, err := s.repo.GetDepartment(ctx, departmentID)
	if err != nil {
		return false, err
	}
	return dept.LeadMemberID != nil && *dept.LeadMemberID == memberID, nil
}
