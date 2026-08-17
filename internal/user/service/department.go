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
