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
	FindMemberByIdentifier(ctx context.Context, identifier string) (types.Member, error)
	ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error)
	UpdateMemberStatus(ctx context.Context, id string, status types.MemberStatus) error
	UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error
	UpdateMemberRole(ctx context.Context, id, roleID string) error
	UpdateMemberProfile(ctx context.Context, id, name string, email, avatarURL *string) error
	UpdateMemberContact(ctx context.Context, id string, email, phone *string) error
}

func (r *repo) CreateMember(ctx context.Context, m types.Member) (types.Member, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO members (org_id, department_id, role_id, name, email, phone, status, avatar_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
		m.OrgID, m.DepartmentID, m.RoleID, m.Name, m.Email, m.Phone, m.Status, m.AvatarURL,
	).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (r *repo) GetMember(ctx context.Context, id string) (types.Member, error) {
	var m types.Member
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, avatar_url, created_at
		FROM members WHERE id = $1`, id,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.AvatarURL, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Member{}, ErrNotFound
	}
	return m, err
}

// FindMemberByIdentifier is how any login method recognises somebody the
// deployment already knows: the address or number is the person, and the
// method -- an IM directory, a one-time code -- is only the proof.
// Without this, whether you can sign in depends on which table happens
// to hold a row for you rather than on who you are.
//
// Email is matched case-insensitively: an address typed into the
// first-run setup form and the same address held by an IM directory
// differ in case often enough to matter.
func (r *repo) FindMemberByIdentifier(ctx context.Context, identifier string) (types.Member, error) {
	var m types.Member
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, avatar_url, created_at
		FROM members WHERE lower(email) = lower($1) OR phone = $1
		ORDER BY created_at
		LIMIT 1`, identifier,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.AvatarURL, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Member{}, ErrNotFound
	}
	return m, err
}

func (r *repo) ListMembers(ctx context.Context, orgID string, departmentID *string) ([]types.Member, error) {
	query := `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, avatar_url, created_at
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
		if err := rows.Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.AvatarURL, &m.CreatedAt); err != nil {
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

// UpdateMemberProfile refreshes the fields an identity source owns, on
// every sign-in rather than only at first sight: a colleague who changes
// their photo or their display name in the IM would otherwise keep the
// one they had the day they first logged in here, forever.
func (r *repo) UpdateMemberProfile(ctx context.Context, id, name string, email, avatarURL *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE members SET name = $1, email = COALESCE($2, email), avatar_url = COALESCE($3, avatar_url) WHERE id = $4`,
		name, email, avatarURL, id,
	)
	return err
}

// UpdateMemberContact sets the address and number outright, nils
// included, so an admin can repair a member whose identity source never
// supplied one -- the alternative for that member is being unable to
// sign in at all once the source is switched off.
func (r *repo) UpdateMemberContact(ctx context.Context, id string, email, phone *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET email = $1, phone = $2 WHERE id = $3`, email, phone, id)
	return err
}
