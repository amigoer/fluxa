// Package quota computes department quota pool balances and backs the
// employee quota-request approval flow (DESIGN.md 7.2, 8.4).
package quota

import "context"

type Store interface {
	GetDepartmentQuotaPoolTotal(ctx context.Context, departmentID string) (int64, error)
	DepartmentQuotaPoolSpoken(ctx context.Context, departmentID string) (int64, error)
}

// Balance is a department quota pool's live state: how much was
// allocated to it, how much active virtual keys already draw from it,
// and what's left. Remaining is not stored anywhere -- see DESIGN.md
// 7.2 "鉴权与一致性" -- it is Total minus Spoken, computed fresh every
// time so it can never drift out of sync with the keys that draw on it.
//
// A negative Remaining is allowed and meaningful, not an error: like
// procurement records (DESIGN.md 8.1), v1 trusts admins and department
// leads to see the true number rather than silently blocking an
// approval that overdraws the pool.
type Balance struct {
	Total     int64
	Spoken    int64
	Remaining int64
}

func GetBalance(ctx context.Context, store Store, departmentID string) (Balance, error) {
	total, err := store.GetDepartmentQuotaPoolTotal(ctx, departmentID)
	if err != nil {
		return Balance{}, err
	}
	spoken, err := store.DepartmentQuotaPoolSpoken(ctx, departmentID)
	if err != nil {
		return Balance{}, err
	}
	return Balance{Total: total, Spoken: spoken, Remaining: total - spoken}, nil
}
