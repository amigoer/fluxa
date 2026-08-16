package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// MemberRepo stores the people in the org, along with the department and
// role each one is placed in.
type MemberRepo interface {
	CreateMember(ctx context.Context, m types.Member) (types.Member, error)
	GetMember(ctx context.Context, id string) (types.Member, error)
	ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error)
	UpdateMemberStatus(ctx context.Context, id string, status types.MemberStatus) error
	UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error
	UpdateMemberRole(ctx context.Context, id, roleID string) error
}

func (r *repo) CreateMember(ctx context.Context, m types.Member) (types.Member, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO members (org_id, department_id, role_id, name, email, phone, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		m.OrgID, m.DepartmentID, m.RoleID, m.Name, m.Email, m.Phone, m.Status,
	).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (r *repo) GetMember(ctx context.Context, id string) (types.Member, error) {
	var m types.Member
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, created_at
		FROM members WHERE id = $1`, id,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Member{}, ErrNotFound
	}
	return m, err
}

func (r *repo) ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error) {
	query := `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, created_at
		FROM members WHERE org_id = $1`
	args := []any{orgID}
	if departmentID != nil {
		query += ` AND department_id = $2`
		args = append(args, *departmentID)
	}
	query += ` ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Member
	for rows.Next() {
		var m types.Member
		if err := rows.Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *repo) UpdateMemberStatus(ctx context.Context, id string, status types.MemberStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *repo) UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET department_id = $1 WHERE id = $2`, departmentID, id)
	return err
}

func (r *repo) UpdateMemberRole(ctx context.Context, id, roleID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET role_id = $1 WHERE id = $2`, roleID, id)
	return err
}
