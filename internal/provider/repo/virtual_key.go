package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// VirtualKeyRepo stores the keys callers authenticate with -- only the
// hash of each secret -- along with the budget every call spends from.
type VirtualKeyRepo interface {
	CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, error)
	FindVirtualKeyByHash(ctx context.Context, secretHash string) (types.VirtualKey, error)
	ListActiveVirtualKeysByMember(ctx context.Context, memberID string) ([]types.VirtualKey, error)
	ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error)
	UpdateVirtualKeyBudget(ctx context.Context, id string, budgetCents int64) error
	RevokeVirtualKey(ctx context.Context, id string) error
	SpendFromVirtualKey(ctx context.Context, id string, amountCents int64) (bool, error)
}

func (r *repo) CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO virtual_keys (name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_cents, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, spent_cents, period_started_at, created_at`,
		k.Name, k.SecretHash, k.SecretPrefix, k.OwnerType, k.OwnerMemberID, k.OwnerDepartmentID, k.ModelScope, k.BudgetCents, k.Status,
	).Scan(&k.ID, &k.SpentCents, &k.PeriodStartedAt, &k.CreatedAt)
	return k, err
}

func (r *repo) FindVirtualKeyByHash(ctx context.Context, secretHash string) (types.VirtualKey, error) {
	var k types.VirtualKey
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_cents, spent_cents, period_started_at, status, created_at, revoked_at
		FROM virtual_keys WHERE secret_hash = $1`, secretHash,
	).Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetCents, &k.SpentCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.VirtualKey{}, ErrNotFound
	}
	return k, err
}

// ListActiveVirtualKeysByMember returns memberID's own active virtual
// keys, used both for the self-service Key list and to find a key to
// grant approved quota onto (see the service package's grantQuota).
func (r *repo) ListActiveVirtualKeysByMember(ctx context.Context, memberID string) ([]types.VirtualKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_cents, spent_cents, period_started_at, status, created_at, revoked_at
		FROM virtual_keys WHERE owner_member_id = $1 AND status = 'active'
		ORDER BY created_at`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.VirtualKey
	for rows.Next() {
		var k types.VirtualKey
		if err := rows.Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetCents, &k.SpentCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *repo) ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error) {
	// Members and departments both belong to a single organization
	// (one org per deployment, DESIGN.md 9), so there is no org filter
	// needed beyond joining through to confirm the owner exists.
	rows, err := r.pool.Query(ctx, `
		SELECT vk.id, vk.name, vk.secret_hash, vk.secret_prefix, vk.owner_type, vk.owner_member_id, vk.owner_department_id, vk.model_scope, vk.budget_cents, vk.spent_cents, vk.period_started_at, vk.status, vk.created_at, vk.revoked_at
		FROM virtual_keys vk
		LEFT JOIN members m ON m.id = vk.owner_member_id
		LEFT JOIN departments d ON d.id = vk.owner_department_id
		WHERE COALESCE(m.org_id, d.org_id) = $1
		ORDER BY vk.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.VirtualKey
	for rows.Next() {
		var k types.VirtualKey
		if err := rows.Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetCents, &k.SpentCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *repo) UpdateVirtualKeyBudget(ctx context.Context, id string, budgetCents int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE virtual_keys SET budget_cents = $1 WHERE id = $2`, budgetCents, id)
	return err
}

func (r *repo) RevokeVirtualKey(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE virtual_keys SET status = 'revoked', revoked_at = now() WHERE id = $1`, id)
	return err
}

// SpendFromVirtualKey atomically increases spent_cents if doing so
// would not exceed budget_cents, in one round trip so concurrent
// requests against the same key can't both succeed past the budget.
// It also lazily rolls the key into a new monthly period when the
// current period has ended, rather than running a separate reset job.
func (r *repo) SpendFromVirtualKey(ctx context.Context, id string, amountCents int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE virtual_keys SET
			spent_cents = CASE
				WHEN date_trunc('month', now()) > period_started_at THEN $2
				ELSE spent_cents + $2
			END,
			period_started_at = CASE
				WHEN date_trunc('month', now()) > period_started_at THEN date_trunc('month', now())
				ELSE period_started_at
			END
		WHERE id = $1
		  AND status = 'active'
		  AND (
			CASE WHEN date_trunc('month', now()) > period_started_at THEN 0 ELSE spent_cents END
		  ) + $2 <= budget_cents`,
		id, amountCents,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
