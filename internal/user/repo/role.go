package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// RoleRepo stores roles and their permission grants -- the built-in four
// and any custom role an org carves out on top of them.
type RoleRepo interface {
	CreateRole(ctx context.Context, orgID, name string, isBuiltin bool) (types.Role, error)
	GetRoleByID(ctx context.Context, id string) (types.Role, error)
	GetRoleByName(ctx context.Context, orgID, name string) (types.Role, error)
	ListRoles(ctx context.Context, orgID string) ([]types.Role, error)
	GrantPermissions(ctx context.Context, roleID string, permissionCodes []string) error
	RolePermissionCodes(ctx context.Context, roleID string) ([]string, error)
}

func (r *repo) CreateRole(ctx context.Context, orgID, name string, isBuiltin bool) (types.Role, error) {
	var role types.Role
	err := r.pool.QueryRow(ctx,
		`INSERT INTO roles (org_id, name, is_builtin) VALUES ($1, $2, $3)
		 RETURNING id, org_id, name, is_builtin, created_at`,
		orgID, name, isBuiltin,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	return role, err
}

func (r *repo) GetRoleByID(ctx context.Context, id string) (types.Role, error) {
	var role types.Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE id = $1`, id,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Role{}, ErrNotFound
	}
	return role, err
}

func (r *repo) GetRoleByName(ctx context.Context, orgID, name string) (types.Role, error) {
	var role types.Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE org_id = $1 AND name = $2`,
		orgID, name,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Role{}, ErrNotFound
	}
	return role, err
}

func (r *repo) ListRoles(ctx context.Context, orgID string) ([]types.Role, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE org_id = $1 ORDER BY is_builtin DESC, name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []types.Role
	for rows.Next() {
		var role types.Role
		if err := rows.Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// GrantPermissions replaces every permission grant for roleID with the
// given codes, in one transaction.
func (r *repo) GrantPermissions(ctx context.Context, roleID string, permissionCodes []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}

	for _, code := range permissionCodes {
		_, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE code = $2`,
			roleID, code,
		)
		if err != nil {
			return fmt.Errorf("user: grant permission %q: %w", code, err)
		}
	}

	return tx.Commit(ctx)
}

// RolePermissionCodes returns the permission codes granted to roleID.
func (r *repo) RolePermissionCodes(ctx context.Context, roleID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.code FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}
