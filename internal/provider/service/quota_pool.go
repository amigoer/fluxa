package service

import (
	"context"
	"fmt"

	"github.com/amigoer/fluxa/internal/provider/quota"
)

// QuotaPoolService is the direct-allocation path to budget: an admin (or
// a department lead within their own pool) sizing a pool or a member's
// key outright, alongside the request-and-approve path in
// QuotaRequestService.
type QuotaPoolService interface {
	SetDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error
	DepartmentQuotaBalance(ctx context.Context, departmentID string) (quota.Balance, error)
	AdjustMemberQuota(ctx context.Context, keyID string, newBudgetCents int64) error
}

func (s *service) SetDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error {
	return s.repo.UpsertDepartmentQuotaPool(ctx, departmentID, totalCents)
}

func (s *service) DepartmentQuotaBalance(ctx context.Context, departmentID string) (quota.Balance, error) {
	return quota.GetBalance(ctx, s.repo, departmentID)
}

// AdjustMemberQuota lets an admin (or a department lead within their own
// pool) directly grant or resize a member's virtual key budget, the
// second path that coexists with quota requests (DESIGN.md 7.2, open
// question #1 as resolved: both paths write the same virtual_keys row,
// department pool balance is derived, so the two can't drift apart).
func (s *service) AdjustMemberQuota(ctx context.Context, keyID string, newBudgetCents int64) error {
	if newBudgetCents < 0 {
		return fmt.Errorf("provider: budget cannot be negative")
	}
	// Implemented as revoke-and-recreate would lose the key's secret;
	// instead this updates budget_cents directly through a minimal
	// repo call kept next to CreateVirtualKey for locality.
	return s.repo.UpdateVirtualKeyBudget(ctx, keyID, newBudgetCents)
}
