package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// QuotaPoolRepo stores each department's total allowance and derives how
// much of it is already spoken for.
type QuotaPoolRepo interface {
	UpsertDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error
	GetDepartmentQuotaPoolTotal(ctx context.Context, departmentID string) (int64, error)
	DepartmentQuotaPoolSpoken(ctx context.Context, departmentID string) (int64, error)
}

func (r *repo) UpsertDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO department_quota_pools (department_id, total_cents, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (department_id) DO UPDATE SET total_cents = EXCLUDED.total_cents, updated_at = now()`,
		departmentID, totalCents,
	)
	return err
}

func (r *repo) GetDepartmentQuotaPoolTotal(ctx context.Context, departmentID string) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT total_cents FROM department_quota_pools WHERE department_id = $1`, departmentID).Scan(&total)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return total, err
}

// DepartmentQuotaPoolSpoken returns the sum of active virtual key
// budgets drawing from departmentID's pool: keys owned directly by the
// department, plus keys owned by members of that department. This is
// the "spoken for" amount subtracted from total_cents to get the live
// balance -- see DESIGN.md 7.2 "鉴权与一致性" for why this is computed
// rather than stored.
func (r *repo) DepartmentQuotaPoolSpoken(ctx context.Context, departmentID string) (int64, error) {
	var spoken int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(vk.budget_cents), 0)
		FROM virtual_keys vk
		LEFT JOIN members m ON m.id = vk.owner_member_id
		WHERE vk.status = 'active'
		  AND (vk.owner_department_id = $1 OR m.department_id = $1)`,
		departmentID,
	).Scan(&spoken)
	return spoken, err
}
