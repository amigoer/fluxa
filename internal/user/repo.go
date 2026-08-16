package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user: not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// -- Organizations -----------------------------------------------------

func (r *Repo) GetOrganization(ctx context.Context) (Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx, `SELECT id, name, created_at FROM organizations LIMIT 1`).
		Scan(&o.ID, &o.Name, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	return o, err
}

func (r *Repo) CreateOrganization(ctx context.Context, name string) (Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, name, created_at`, name,
	).Scan(&o.ID, &o.Name, &o.CreatedAt)
	return o, err
}

// -- Roles --------------------------------------------------------------

func (r *Repo) CreateRole(ctx context.Context, orgID, name string, isBuiltin bool) (Role, error) {
	var role Role
	err := r.pool.QueryRow(ctx,
		`INSERT INTO roles (org_id, name, is_builtin) VALUES ($1, $2, $3)
		 RETURNING id, org_id, name, is_builtin, created_at`,
		orgID, name, isBuiltin,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	return role, err
}

func (r *Repo) GetRoleByID(ctx context.Context, id string) (Role, error) {
	var role Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE id = $1`, id,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

func (r *Repo) GetRoleByName(ctx context.Context, orgID, name string) (Role, error) {
	var role Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE org_id = $1 AND name = $2`,
		orgID, name,
	).Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

func (r *Repo) ListRoles(ctx context.Context, orgID string) ([]Role, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, is_builtin, created_at FROM roles WHERE org_id = $1 ORDER BY is_builtin DESC, name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.OrgID, &role.Name, &role.IsBuiltin, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// GrantPermissions replaces every permission grant for roleID with the
// given codes, in one transaction.
func (r *Repo) GrantPermissions(ctx context.Context, roleID string, permissionCodes []string) error {
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
func (r *Repo) RolePermissionCodes(ctx context.Context, roleID string) ([]string, error) {
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

// -- Departments ----------------------------------------------------------

func (r *Repo) CreateDepartment(ctx context.Context, orgID, name string) (Department, error) {
	var d Department
	err := r.pool.QueryRow(ctx,
		`INSERT INTO departments (org_id, name) VALUES ($1, $2)
		 RETURNING id, org_id, name, lead_member_id, created_at`,
		orgID, name,
	).Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt)
	return d, err
}

func (r *Repo) GetDepartment(ctx context.Context, id string) (Department, error) {
	var d Department
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, lead_member_id, created_at FROM departments WHERE id = $1`, id,
	).Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Department{}, ErrNotFound
	}
	return d, err
}

func (r *Repo) ListDepartments(ctx context.Context, orgID string) ([]Department, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, lead_member_id, created_at FROM departments WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Name, &d.LeadMemberID, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) SetDepartmentLead(ctx context.Context, departmentID string, leadMemberID *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE departments SET lead_member_id = $1 WHERE id = $2`, leadMemberID, departmentID)
	return err
}

// -- Members ----------------------------------------------------------

func (r *Repo) CreateMember(ctx context.Context, m Member) (Member, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO members (org_id, department_id, role_id, name, email, phone, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		m.OrgID, m.DepartmentID, m.RoleID, m.Name, m.Email, m.Phone, m.Status,
	).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (r *Repo) GetMember(ctx context.Context, id string) (Member, error) {
	var m Member
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, department_id, role_id, name, email, phone, status, created_at
		FROM members WHERE id = $1`, id,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

func (r *Repo) ListMembers(ctx context.Context, orgID string, departmentID *string) ([]Member, error) {
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

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateMemberStatus(ctx context.Context, id string, status MemberStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *Repo) UpdateMemberDepartment(ctx context.Context, id string, departmentID *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET department_id = $1 WHERE id = $2`, departmentID, id)
	return err
}

func (r *Repo) UpdateMemberRole(ctx context.Context, id, roleID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE members SET role_id = $1 WHERE id = $2`, roleID, id)
	return err
}

// -- External identities & identity source config -----------------------

func (r *Repo) FindMemberByExternalIdentity(ctx context.Context, provider IdentityProvider, externalUserID string) (Member, error) {
	var m Member
	err := r.pool.QueryRow(ctx, `
		SELECT m.id, m.org_id, m.department_id, m.role_id, m.name, m.email, m.phone, m.status, m.created_at
		FROM members m
		JOIN external_identities ei ON ei.member_id = m.id
		WHERE ei.provider = $1 AND ei.external_user_id = $2`,
		provider, externalUserID,
	).Scan(&m.ID, &m.OrgID, &m.DepartmentID, &m.RoleID, &m.Name, &m.Email, &m.Phone, &m.Status, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

func (r *Repo) LinkExternalIdentity(ctx context.Context, memberID string, provider IdentityProvider, externalUserID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO external_identities (member_id, provider, external_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, external_user_id) DO NOTHING`,
		memberID, provider, externalUserID,
	)
	return err
}

func (r *Repo) GetIdentityConfig(ctx context.Context, provider IdentityProvider) (IdentityConfig, error) {
	var c IdentityConfig
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, app_id, app_secret, callback_path, enabled, created_at, updated_at
		FROM identity_configs WHERE provider = $1`, provider,
	).Scan(&c.ID, &c.Provider, &c.AppID, &c.AppSecret, &c.CallbackPath, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdentityConfig{}, ErrNotFound
	}
	return c, err
}

func (r *Repo) UpsertIdentityConfig(ctx context.Context, c IdentityConfig) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO identity_configs (provider, app_id, app_secret, callback_path, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (provider) DO UPDATE SET
			app_id = EXCLUDED.app_id,
			app_secret = EXCLUDED.app_secret,
			callback_path = EXCLUDED.callback_path,
			enabled = EXCLUDED.enabled,
			updated_at = now()`,
		c.Provider, c.AppID, c.AppSecret, c.CallbackPath, c.Enabled,
	)
	return err
}

// -- Auth settings & local accounts --------------------------------------

func (r *Repo) GetAuthSettings(ctx context.Context) (AuthSettings, error) {
	var s AuthSettings
	err := r.pool.QueryRow(ctx,
		`SELECT local_account_enabled, local_account_requires_approval FROM auth_settings`,
	).Scan(&s.LocalAccountEnabled, &s.LocalAccountRequiresApproval)
	return s, err
}

func (r *Repo) UpdateAuthSettings(ctx context.Context, s AuthSettings) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth_settings SET local_account_enabled = $1, local_account_requires_approval = $2`,
		s.LocalAccountEnabled, s.LocalAccountRequiresApproval,
	)
	return err
}

func (r *Repo) CreateLocalAccount(ctx context.Context, a LocalAccount) (LocalAccount, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO local_accounts (member_id, phone, email)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		a.MemberID, a.Phone, a.Email,
	).Scan(&a.ID, &a.CreatedAt)
	return a, err
}

func (r *Repo) FindLocalAccountByIdentifier(ctx context.Context, identifier string) (LocalAccount, error) {
	var a LocalAccount
	err := r.pool.QueryRow(ctx, `
		SELECT id, member_id, phone, email, created_at
		FROM local_accounts WHERE phone = $1 OR email = $1`, identifier,
	).Scan(&a.ID, &a.MemberID, &a.Phone, &a.Email, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LocalAccount{}, ErrNotFound
	}
	return a, err
}

// CreateOTP stores a new one-time code, hashed, for identifier/purpose.
// CountOTPsSince reports how many codes were issued to identifier since
// `since`, and when the most recent one went out. Both come from one
// query because the caller needs both to answer "may I send another?".
//
// This is the durable half of the OTP rate limit: it survives a restart
// and is shared across replicas, which the in-process limiter is not.
func (r *Repo) CountOTPsSince(ctx context.Context, identifier string, since time.Time) (int, time.Time, error) {
	var count int
	var latest *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT count(*), max(created_at) FROM local_otp_codes
		WHERE identifier = $1 AND created_at >= $2`,
		identifier, since,
	).Scan(&count, &latest)
	if err != nil {
		return 0, time.Time{}, err
	}
	if latest == nil {
		return count, time.Time{}, nil
	}
	return count, *latest, nil
}

func (r *Repo) CreateOTP(ctx context.Context, identifier string, purpose OTPPurpose, codeHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO local_otp_codes (identifier, purpose, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		identifier, purpose, codeHash, expiresAt,
	)
	return err
}

// maxOTPAttempts is how many guesses one code will tolerate before it is
// spent. Five is comfortably more than a person mistypes and nowhere near
// enough to search a six-digit space.
const maxOTPAttempts = 5

// ConsumeOTP atomically marks the newest live code for identifier/purpose
// as consumed if its hash matches, returning whether it matched. Using
// UPDATE ... RETURNING makes the check-and-consume a single round trip so
// two concurrent attempts can't both succeed.
//
// The row is selected without matching the hash, and every attempt --
// right or wrong -- increments the counter, which is what makes a wrong
// guess cost something. Selecting by hash instead (as this did before)
// meant a wrong guess simply found no row and could be repeated forever.
func (r *Repo) ConsumeOTP(ctx context.Context, identifier string, purpose OTPPurpose, codeHash string) (bool, error) {
	var matched bool
	err := r.pool.QueryRow(ctx, `
		UPDATE local_otp_codes
		SET attempts = attempts + 1,
		    consumed_at = CASE WHEN code_hash = $3 THEN now() ELSE consumed_at END
		WHERE id = (
			SELECT id FROM local_otp_codes
			WHERE identifier = $1 AND purpose = $2
			  AND consumed_at IS NULL AND expires_at > now()
			  AND attempts < $4
			ORDER BY created_at DESC
			LIMIT 1
		)
		RETURNING code_hash = $3`,
		identifier, purpose, codeHash, maxOTPAttempts,
	).Scan(&matched)
	if errors.Is(err, pgx.ErrNoRows) {
		// No live code left: never issued, already used, expired, or its
		// attempts are spent. All of these are "invalid code" to the
		// caller -- distinguishing them would say more than it should.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return matched, nil
}

// -- Sessions -----------------------------------------------------------

// SessionRecord is the row shape sessions are stored/read as; token_hash
// is sha256(raw token), never the raw token itself (DESIGN.md 7.1).
type SessionRecord struct {
	TokenHash string
	MemberID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (r *Repo) CreateSession(ctx context.Context, tokenHash, memberID string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, member_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, memberID, expiresAt,
	)
	return err
}

func (r *Repo) GetSession(ctx context.Context, tokenHash string) (SessionRecord, error) {
	var s SessionRecord
	err := r.pool.QueryRow(ctx,
		`SELECT token_hash, member_id, expires_at, revoked_at FROM sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&s.TokenHash, &s.MemberID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRecord{}, ErrNotFound
	}
	return s, err
}

func (r *Repo) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}

// RevokeAllSessionsForMember lets an admin force a member's active
// sessions to end immediately -- the whole point of choosing a
// server-side session over a stateless JWT (DESIGN.md 7.1).
func (r *Repo) RevokeAllSessionsForMember(ctx context.Context, memberID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE member_id = $1 AND revoked_at IS NULL`,
		memberID,
	)
	return err
}

// -- Notify channels ----------------------------------------------------

func (r *Repo) GetNotifyChannel(ctx context.Context, kind NotifyChannelKind) (NotifyChannel, error) {
	var c NotifyChannel
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, kind, provider, config, enabled, created_at, updated_at
		FROM notify_channels WHERE kind = $1`, kind,
	).Scan(&c.ID, &c.Kind, &c.Provider, &raw, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotifyChannel{}, ErrNotFound
	}
	if err != nil {
		return NotifyChannel{}, err
	}
	if err := json.Unmarshal(raw, &c.Config); err != nil {
		return NotifyChannel{}, fmt.Errorf("user: decode notify channel config: %w", err)
	}
	return c, nil
}

func (r *Repo) UpsertNotifyChannel(ctx context.Context, c NotifyChannel) error {
	raw, err := json.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("user: encode notify channel config: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO notify_channels (kind, provider, config, enabled, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (kind) DO UPDATE SET
			provider = EXCLUDED.provider,
			config = EXCLUDED.config,
			enabled = EXCLUDED.enabled,
			updated_at = now()`,
		c.Kind, c.Provider, raw, c.Enabled,
	)
	return err
}

// Outcomes recorded in notify_log.
const (
	NotifyStatusSent   = "sent"
	NotifyStatusFailed = "failed"
)

// failureTextLimit keeps one pathological upstream message from bloating
// the row. Every SMTP error worth reading fits well inside it.
const failureTextLimit = 500

// LogNotifySent records a delivered message.
func (r *Repo) LogNotifySent(ctx context.Context, kind NotifyChannelKind, recipient, purpose string) error {
	return r.logNotify(ctx, kind, recipient, purpose, NotifyStatusSent, "")
}

// LogNotifyFailed records an attempt the vendor rejected, with the reason.
// Failures were previously not recorded at all, which left "the code never
// arrived" with nothing on the server side to look at.
func (r *Repo) LogNotifyFailed(ctx context.Context, kind NotifyChannelKind, recipient, purpose string, cause error) error {
	text := cause.Error()
	if len(text) > failureTextLimit {
		text = text[:failureTextLimit]
	}
	return r.logNotify(ctx, kind, recipient, purpose, NotifyStatusFailed, text)
}

func (r *Repo) logNotify(ctx context.Context, kind NotifyChannelKind, recipient, purpose, status, failure string) error {
	var errText *string
	if failure != "" {
		errText = &failure
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notify_log (kind, recipient, purpose, status, error) VALUES ($1, $2, $3, $4, $5)`,
		kind, recipient, purpose, status, errText,
	)
	return err
}

// CountNotifySentThisMonth counts delivered messages only: a failed
// attempt costs no quota at the vendor and should not read as usage.
func (r *Repo) CountNotifySentThisMonth(ctx context.Context, kind NotifyChannelKind) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM notify_log
		WHERE kind = $1 AND status = $2 AND sent_at >= date_trunc('month', now())`,
		kind, NotifyStatusSent,
	).Scan(&n)
	return n, err
}
