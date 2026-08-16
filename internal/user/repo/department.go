package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// DepartmentRepo stores the org's departments and who leads each one --
// the lead being what the provider module's quota approval routes to.
type DepartmentRepo interface {
	CreateDepartment(ctx context.Context, orgID, name string) (types.Department, error)
	GetDepartment(ctx context.Context, id string) (types.Department, error)
	ListDepartments(ctx context.Context, orgID string) ([]types.Department, error)
	SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error
}

func (r *repo) CreateDepartment(ctx context.Context, orgID, name string) (types.Department, error) {
	var d types.Department
	err := r.pool.QueryRow(ctx,
		`INSERT INTO departments (org_id, name) VALUES ($1, $2)
		 RETURNING id, org_id, name, lead_member_id, created_at`,
		orgID, name,
	).Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt)
	return d, err
}

func (r *repo) GetDepartment(ctx context.Context, id string) (types.Department, error) {
	var d types.Department
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, lead_member_id, created_at FROM departments WHERE id = $1`, id,
	).Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Department{}, ErrNotFound
	}
	return d, err
}

func (r *repo) ListDepartments(ctx context.Context, orgID string) ([]types.Department, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, lead_member_id, created_at FROM departments WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Department
	for rows.Next() {
		var d types.Department
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repo) SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE departments SET lead_member_id = $1 WHERE id = $2`, leadMemberID, departmentID)
	return err
}
