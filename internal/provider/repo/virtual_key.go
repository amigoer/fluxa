package repo

import (
	"context"
	"errors"
	"time"

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
	UpdateVirtualKeyBudget(ctx context.Context, id string, budgetMicroCents int64) error
	RevokeVirtualKey(ctx context.Context, id string) (string, error)
	ReserveFromVirtualKey(ctx context.Context, id string, amountMicroCents int64, ttl time.Duration) (string, bool, error)
	SettleReservation(ctx context.Context, reservationID string, actualMicroCents int64) error
	ExpireStaleReservations(ctx context.Context) (int64, error)
}

func (r *repo) CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO virtual_keys (name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_micro_cents, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, spent_micro_cents, period_started_at, created_at`,
		k.Name, k.SecretHash, k.SecretPrefix, k.OwnerType, k.OwnerMemberID, k.OwnerDepartmentID, k.ModelScope, k.BudgetMicroCents, k.Status,
	).Scan(&k.ID, &k.SpentMicroCents, &k.PeriodStartedAt, &k.CreatedAt)
	return k, err
}

func (r *repo) FindVirtualKeyByHash(ctx context.Context, secretHash string) (types.VirtualKey, error) {
	var k types.VirtualKey
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_micro_cents, spent_micro_cents, reserved_micro_cents, period_started_at, status, created_at, revoked_at
		FROM virtual_keys WHERE secret_hash = $1`, secretHash,
	).Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetMicroCents, &k.SpentMicroCents, &k.ReservedMicroCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt)
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
		SELECT id, name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_micro_cents, spent_micro_cents, reserved_micro_cents, period_started_at, status, created_at, revoked_at
		FROM virtual_keys WHERE owner_member_id = $1 AND status = 'active'
		ORDER BY created_at`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.VirtualKey
	for rows.Next() {
		var k types.VirtualKey
		if err := rows.Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetMicroCents, &k.SpentMicroCents, &k.ReservedMicroCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt); err != nil {
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
		SELECT vk.id, vk.name, vk.secret_hash, vk.secret_prefix, vk.owner_type, vk.owner_member_id, vk.owner_department_id, vk.model_scope, vk.budget_micro_cents, vk.spent_micro_cents, vk.reserved_micro_cents, vk.period_started_at, vk.status, vk.created_at, vk.revoked_at
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
		if err := rows.Scan(&k.ID, &k.Name, &k.SecretHash, &k.SecretPrefix, &k.OwnerType, &k.OwnerMemberID, &k.OwnerDepartmentID, &k.ModelScope, &k.BudgetMicroCents, &k.SpentMicroCents, &k.ReservedMicroCents, &k.PeriodStartedAt, &k.Status, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *repo) UpdateVirtualKeyBudget(ctx context.Context, id string, budgetMicroCents int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE virtual_keys SET budget_micro_cents = $1 WHERE id = $2`, budgetMicroCents, id)
	return err
}

// RevokeVirtualKey retires a key and returns the secret hash it was
// stored under, so the caller can drop it from the authentication cache
// rather than leaving it usable until the entry lapses.
//
// Revoking an already-revoked key succeeds and reports the same hash:
// "make sure this key is dead" is the request, and answering an admin
// who clicks it twice with an error would be answering a different
// question. revoked_at keeps the first revocation's timestamp, which is
// the one the audit trail cares about. Only an id that matches no row at
// all is ErrNotFound.
func (r *repo) RevokeVirtualKey(ctx context.Context, id string) (string, error) {
	var secretHash string
	err := r.pool.QueryRow(ctx, `
		UPDATE virtual_keys
		SET status = 'revoked', revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
		RETURNING secret_hash`, id).Scan(&secretHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return secretHash, err
}

// ReserveFromVirtualKey admits a call by setting aside amountMicroCents
// against id's remaining budget, returning the reservation to settle it
// with. ok is false when the key cannot cover it, in which case nothing
// was reserved and the call must not be made.
//
// This runs *before* the upstream request, which is the whole point.
// Spend used to be deducted after the response had already been streamed
// back, so the budget check happened once the money was gone: exceeding
// it produced an audit line and nothing else, and N concurrent calls
// each passed their own check against the same untouched balance. The
// admission test here is spent + reserved + amount <= budget, and it is
// a single guarded UPDATE, so concurrent callers serialize on the row
// and only as many as fit are let through.
//
// The insert is chained off the update in one statement: when the update
// matches no row the insert has nothing to select from, so a refused
// call cannot leave a reservation behind.
func (r *repo) ReserveFromVirtualKey(ctx context.Context, id string, amountMicroCents int64, ttl time.Duration) (string, bool, error) {
	var reservationID string
	err := r.pool.QueryRow(ctx, `
		WITH admitted AS (
			UPDATE virtual_keys SET
				spent_micro_cents = CASE
					WHEN date_trunc('month', now()) > period_started_at THEN 0
					ELSE spent_micro_cents
				END,
				period_started_at = CASE
					WHEN date_trunc('month', now()) > period_started_at THEN date_trunc('month', now())
					ELSE period_started_at
				END,
				reserved_micro_cents = reserved_micro_cents + $2
			WHERE id = $1
			  AND status = 'active'
			  AND (
				CASE WHEN date_trunc('month', now()) > period_started_at THEN 0 ELSE spent_micro_cents END
			  ) + reserved_micro_cents + $2 <= budget_micro_cents
			RETURNING id
		)
		INSERT INTO quota_reservations (virtual_key_id, amount_micro_cents, expires_at)
		SELECT id, $2, now() + $3::interval FROM admitted
		RETURNING id`,
		id, amountMicroCents, ttl.String(),
	).Scan(&reservationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return reservationID, true, nil
}

// SettleReservation closes out a reservation: it releases what was set
// aside and charges what the call actually cost. Pass 0 to release
// without charging, which is what a failed call does.
//
// actualMicroCents may exceed what was reserved -- real usage can come
// back above the estimate the call was admitted on -- and it is recorded
// as-is rather than clamped. Spend is a record of what happened; letting
// it land above budget is what makes the *next* reservation refuse, and
// clamping it would quietly forgive the overrun instead.
func (r *repo) SettleReservation(ctx context.Context, reservationID string, actualMicroCents int64) error {
	_, err := r.pool.Exec(ctx, `
		WITH released AS (
			DELETE FROM quota_reservations WHERE id = $1
			RETURNING virtual_key_id, amount_micro_cents
		)
		UPDATE virtual_keys vk SET
			spent_micro_cents = CASE
				WHEN date_trunc('month', now()) > vk.period_started_at THEN $2
				ELSE vk.spent_micro_cents + $2
			END,
			period_started_at = CASE
				WHEN date_trunc('month', now()) > vk.period_started_at THEN date_trunc('month', now())
				ELSE vk.period_started_at
			END,
			reserved_micro_cents = GREATEST(vk.reserved_micro_cents - released.amount_micro_cents, 0)
		FROM released
		WHERE vk.id = released.virtual_key_id`,
		reservationID, actualMicroCents,
	)
	return err
}

// ExpireStaleReservations releases everything past its expiry and
// reports how many it freed. A process killed mid-call leaves its
// reservation behind, and budget promised to a call that will never
// settle would otherwise strangle the key permanently -- so this is a
// correctness requirement of the reservation scheme, not housekeeping.
func (r *repo) ExpireStaleReservations(ctx context.Context) (int64, error) {
	var freed int64
	err := r.pool.QueryRow(ctx, `
		WITH expired AS (
			DELETE FROM quota_reservations WHERE expires_at < now()
			RETURNING virtual_key_id, amount_micro_cents
		), per_key AS (
			SELECT virtual_key_id, SUM(amount_micro_cents) AS total
			FROM expired GROUP BY virtual_key_id
		), rolled_back AS (
			UPDATE virtual_keys vk
			SET reserved_micro_cents = GREATEST(vk.reserved_micro_cents - per_key.total, 0)
			FROM per_key WHERE vk.id = per_key.virtual_key_id
			RETURNING vk.id
		)
		SELECT count(*) FROM expired`).Scan(&freed)
	return freed, err
}
