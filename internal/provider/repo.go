package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amigoer/fluxa/internal/provider/types"
)

var ErrNotFound = errors.New("provider: not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// -- Providers ------------------------------------------------------------

func (r *Repo) CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error) {
	raw, err := json.Marshal(p.Config)
	if err != nil {
		return types.Provider{}, err
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO providers (org_id, name, kind, config, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		p.OrgID, p.Name, p.Kind, raw, p.Status,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *Repo) GetProvider(ctx context.Context, id string) (types.Provider, error) {
	var p types.Provider
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, name, kind, config, status, created_at, updated_at
		FROM providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Kind, &raw, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Provider{}, ErrNotFound
	}
	if err != nil {
		return types.Provider{}, err
	}
	if err := json.Unmarshal(raw, &p.Config); err != nil {
		return types.Provider{}, err
	}
	return p, nil
}

func (r *Repo) ListProviders(ctx context.Context, orgID string) ([]types.Provider, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, org_id, name, kind, config, status, created_at, updated_at
		FROM providers WHERE org_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Provider
	for rows.Next() {
		var p types.Provider
		var raw []byte
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Kind, &raw, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &p.Config); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateProviderStatus(ctx context.Context, id string, status types.ProviderStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE providers SET status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}

// -- Models -----------------------------------------------------------

func (r *Repo) CreateModel(ctx context.Context, m types.Model) (types.Model, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO models (provider_id, name, model_identifier, status, input_price_cents_per_1m, output_price_cents_per_1m, context_window)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		m.ProviderID, m.Name, m.ModelIdentifier, m.Status, m.InputPriceCentsPer1M, m.OutputPriceCentsPer1M, m.ContextWindow,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (r *Repo) GetModel(ctx context.Context, id string) (types.Model, error) {
	var m types.Model
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider_id, name, model_identifier, status, input_price_cents_per_1m, output_price_cents_per_1m, context_window, created_at, updated_at
		FROM models WHERE id = $1`, id,
	).Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.Model{}, ErrNotFound
	}
	return m, err
}

func (r *Repo) ListModels(ctx context.Context, orgID string) ([]types.Model, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.provider_id, m.name, m.model_identifier, m.status, m.input_price_cents_per_1m, m.output_price_cents_per_1m, m.context_window, m.created_at, m.updated_at
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.org_id = $1
		ORDER BY m.created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Model
	for rows.Next() {
		var m types.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.provider_id, m.name, m.model_identifier, m.status, m.input_price_cents_per_1m, m.output_price_cents_per_1m, m.context_window, m.created_at, m.updated_at, p.kind
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.org_id = $1 AND m.status = 'published' AND p.status = 'active'
		ORDER BY m.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Model
	for rows.Next() {
		var m types.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelIdentifier, &m.Status, &m.InputPriceCentsPer1M, &m.OutputPriceCentsPer1M, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt, &m.ProviderKind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// -- Procurement ----------------------------------------------------------

func (r *Repo) RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO procurement_records (provider_id, amount_cents, note, recorded_by_member_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, recorded_at`,
		rec.ProviderID, rec.AmountCents, rec.Note, rec.RecordedByMemberID,
	).Scan(&rec.ID, &rec.RecordedAt)
	return rec, err
}

func (r *Repo) ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pr.id, pr.provider_id, pr.amount_cents, pr.note, pr.recorded_by_member_id, pr.recorded_at
		FROM procurement_records pr
		JOIN providers p ON p.id = pr.provider_id
		WHERE p.org_id = $1
		ORDER BY pr.recorded_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.ProcurementRecord
	for rows.Next() {
		var rec types.ProcurementRecord
		if err := rows.Scan(&rec.ID, &rec.ProviderID, &rec.AmountCents, &rec.Note, &rec.RecordedByMemberID, &rec.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// -- Routing rules --------------------------------------------------------

func (r *Repo) CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO routing_rules (scope, owner_member_id, condition_label, target_model_id, fallback_model_id, cost_ceiling_cents, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		rule.Scope, rule.OwnerMemberID, rule.ConditionLabel, rule.TargetModelID, rule.FallbackModelID, rule.CostCeilingCents, rule.SortOrder,
	).Scan(&rule.ID, &rule.CreatedAt)
	return rule, err
}

// ListRoutingChain returns the rules for a scope (global, or personal
// for a specific member) in chain order, for routing.Resolver to walk.
func (r *Repo) ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error) {
	query := `
		SELECT id, scope, owner_member_id, condition_label, target_model_id, fallback_model_id, cost_ceiling_cents, sort_order, created_at
		FROM routing_rules WHERE scope = $1`
	args := []any{scope}
	if ownerMemberID != nil {
		query += ` AND owner_member_id = $2`
		args = append(args, *ownerMemberID)
	} else {
		query += ` AND owner_member_id IS NULL`
	}
	query += ` ORDER BY sort_order`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.RoutingRule
	for rows.Next() {
		var rule types.RoutingRule
		if err := rows.Scan(&rule.ID, &rule.Scope, &rule.OwnerMemberID, &rule.ConditionLabel, &rule.TargetModelID, &rule.FallbackModelID, &rule.CostCeilingCents, &rule.SortOrder, &rule.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// -- Virtual keys -----------------------------------------------------

func (r *Repo) CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO virtual_keys (name, secret_hash, secret_prefix, owner_type, owner_member_id, owner_department_id, model_scope, budget_cents, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, spent_cents, period_started_at, created_at`,
		k.Name, k.SecretHash, k.SecretPrefix, k.OwnerType, k.OwnerMemberID, k.OwnerDepartmentID, k.ModelScope, k.BudgetCents, k.Status,
	).Scan(&k.ID, &k.SpentCents, &k.PeriodStartedAt, &k.CreatedAt)
	return k, err
}

func (r *Repo) FindVirtualKeyByHash(ctx context.Context, secretHash string) (types.VirtualKey, error) {
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
// grant approved quota onto (see Service.grantQuota).
func (r *Repo) ListActiveVirtualKeysByMember(ctx context.Context, memberID string) ([]types.VirtualKey, error) {
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

func (r *Repo) ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error) {
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

func (r *Repo) updateVirtualKeyBudget(ctx context.Context, id string, budgetCents int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE virtual_keys SET budget_cents = $1 WHERE id = $2`, budgetCents, id)
	return err
}

func (r *Repo) RevokeVirtualKey(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE virtual_keys SET status = 'revoked', revoked_at = now() WHERE id = $1`, id)
	return err
}

// SpendFromVirtualKey atomically increases spent_cents if doing so
// would not exceed budget_cents, in one round trip so concurrent
// requests against the same key can't both succeed past the budget.
// It also lazily rolls the key into a new monthly period when the
// current period has ended, rather than running a separate reset job.
func (r *Repo) SpendFromVirtualKey(ctx context.Context, id string, amountCents int64) (bool, error) {
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

// -- Department quota pools ----------------------------------------------

func (r *Repo) UpsertDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO department_quota_pools (department_id, total_cents, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (department_id) DO UPDATE SET total_cents = EXCLUDED.total_cents, updated_at = now()`,
		departmentID, totalCents,
	)
	return err
}

func (r *Repo) GetDepartmentQuotaPoolTotal(ctx context.Context, departmentID string) (int64, error) {
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
func (r *Repo) DepartmentQuotaPoolSpoken(ctx context.Context, departmentID string) (int64, error) {
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

// -- Quota requests -----------------------------------------------------

func (r *Repo) CreateQuotaRequest(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO quota_requests (requested_by_member_id, model_id, amount_cents, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`,
		q.RequestedByMemberID, q.ModelID, q.AmountCents, q.Reason,
	).Scan(&q.ID, &q.Status, &q.CreatedAt)
	return q, err
}

func (r *Repo) GetQuotaRequest(ctx context.Context, id string) (types.QuotaRequest, error) {
	var q types.QuotaRequest
	err := r.pool.QueryRow(ctx, `
		SELECT id, requested_by_member_id, model_id, amount_cents, reason, status, decided_by_member_id, decided_at, created_at
		FROM quota_requests WHERE id = $1`, id,
	).Scan(&q.ID, &q.RequestedByMemberID, &q.ModelID, &q.AmountCents, &q.Reason, &q.Status, &q.DecidedByMemberID, &q.DecidedAt, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.QuotaRequest{}, ErrNotFound
	}
	return q, err
}

func (r *Repo) ListQuotaRequestsByMember(ctx context.Context, memberID string) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `WHERE requested_by_member_id = $1`, memberID)
}

func (r *Repo) ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `
		WHERE status = 'pending' AND requested_by_member_id IN (
			SELECT id FROM members WHERE department_id = $1
		)`, departmentID)
}

func (r *Repo) ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error) {
	return r.queryQuotaRequests(ctx, `WHERE status = 'pending'`)
}

func (r *Repo) queryQuotaRequests(ctx context.Context, where string, args ...any) ([]types.QuotaRequest, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, requested_by_member_id, model_id, amount_cents, reason, status, decided_by_member_id, decided_at, created_at
		FROM quota_requests %s ORDER BY created_at DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.QuotaRequest
	for rows.Next() {
		var q types.QuotaRequest
		if err := rows.Scan(&q.ID, &q.RequestedByMemberID, &q.ModelID, &q.AmountCents, &q.Reason, &q.Status, &q.DecidedByMemberID, &q.DecidedAt, &q.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// CanApproveQuotaRequest reports whether deciderMemberID is the lead of
// the department the requester in requestID belongs to. It is a single
// SQL join across members/departments rather than a cross-module Go
// call into the user package: modules share one database, and this
// keeps provider decoupled from user's internals for a check this
// narrow. The admin fallback (quota.approve_any) is checked separately,
// against the caller's rbac.Principal, since that doesn't need a query
// at all.
func (r *Repo) CanApproveQuotaRequest(ctx context.Context, requestID, deciderMemberID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM quota_requests qr
			JOIN members m ON m.id = qr.requested_by_member_id
			JOIN departments d ON d.id = m.department_id
			WHERE qr.id = $1 AND d.lead_member_id = $2
		)`, requestID, deciderMemberID,
	).Scan(&ok)
	return ok, err
}

func (r *Repo) DecideQuotaRequest(ctx context.Context, id string, status types.QuotaRequestStatus, decidedByMemberID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE quota_requests SET status = $1, decided_by_member_id = $2, decided_at = now()
		WHERE id = $3 AND status = 'pending'`,
		status, decidedByMemberID, id,
	)
	return err
}

// -- types.Provider health ------------------------------------------------------

func (r *Repo) GetProviderHealth(ctx context.Context, providerID string) (types.ProviderHealth, error) {
	var h types.ProviderHealth
	err := r.pool.QueryRow(ctx, `
		INSERT INTO provider_health_states (provider_id) VALUES ($1)
		ON CONFLICT (provider_id) DO UPDATE SET provider_id = EXCLUDED.provider_id
		RETURNING provider_id, state, consecutive_failures, opened_at, last_probe_at, updated_at`,
		providerID,
	).Scan(&h.ProviderID, &h.State, &h.ConsecutiveFailures, &h.OpenedAt, &h.LastProbeAt, &h.UpdatedAt)
	return h, err
}

func (r *Repo) SaveProviderHealth(ctx context.Context, h types.ProviderHealth) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_health_states SET
			state = $1, consecutive_failures = $2, opened_at = $3, last_probe_at = $4, updated_at = now()
		WHERE provider_id = $5`,
		h.State, h.ConsecutiveFailures, h.OpenedAt, h.LastProbeAt, h.ProviderID,
	)
	return err
}
