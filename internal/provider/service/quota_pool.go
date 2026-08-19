package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/quota"
)

// QuotaPoolService is the department-pool half of budget management:
// sizing a pool, and reading what is left in it. The request-and-approve
// path that actually draws from a pool is QuotaRequestService.
type QuotaPoolService interface {
	SetDepartmentQuotaPool(ctx context.Context, departmentID string, totalMicroCents int64) error
	DepartmentQuotaBalance(ctx context.Context, departmentID string) (quota.Balance, error)
}

func (s *service) SetDepartmentQuotaPool(ctx context.Context, departmentID string, totalMicroCents int64) error {
	return s.repo.UpsertDepartmentQuotaPool(ctx, departmentID, totalMicroCents)
}

func (s *service) DepartmentQuotaBalance(ctx context.Context, departmentID string) (quota.Balance, error) {
	return quota.GetBalance(ctx, s.repo, departmentID)
}
