// keys.go — virtual key and usage record persistence.
//
// A VirtualKey is the credential an application presents instead of the
// upstream provider key. It carries authorisation (model allowlist, IP
// allowlist, expiry) and rate / budget limits. Usage records link back to
// the virtual key so per-key accounting becomes a simple SQL aggregate.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// VirtualKey mirrors the virtual_keys table row. All budget fields are 0
// to mean "unlimited"; ExpiresAt is a nil pointer for "never".
type VirtualKey struct {
	ID                  string
	Name                string
	Description         string
	Models              []string // empty = all models allowed
	IPAllowlist         []string // empty = any IP allowed
	BudgetTokensDaily   int64
	BudgetTokensMonthly int64
	BudgetUSDDaily      float64
	BudgetUSDMonthly    float64
	RPMLimit            int
	Enabled             bool
	ExpiresAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UsageRecord is a single row in usage_records, one per successful or
// failed chat call attributed to a virtual key.
type UsageRecord struct {
	ID               int64
	VirtualKeyID     string
	Ts               time.Time
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostUSD          float64
	LatencyMs        int
	Status           int
}

// UsageTotals aggregates usage over a time window for a single virtual key.
type UsageTotals struct {
	Tokens   int64
	PromptTokens int64
	CompletionTokens int64
	CostUSD  float64
	Requests int64
}

// -- virtual key CRUD --------------------------------------------------

// ListVirtualKeys returns every virtual key row ordered by created_at.
func (s *Store) ListVirtualKeys(ctx context.Context) ([]VirtualKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, models, ip_allowlist,
		       budget_tokens_daily, budget_tokens_monthly,
		       budget_usd_daily, budget_usd_monthly, rpm_limit,
		       enabled, expires_at, created_at, updated_at
		FROM virtual_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VirtualKey
	for rows.Next() {
		vk, err := scanVirtualKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, vk)
	}
	return out, rows.Err()
}

// GetVirtualKey loads one row by id. Returns ErrNotFound on miss.
func (s *Store) GetVirtualKey(ctx context.Context, id string) (VirtualKey, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, models, ip_allowlist,
		       budget_tokens_daily, budget_tokens_monthly,
		       budget_usd_daily, budget_usd_monthly, rpm_limit,
		       enabled, expires_at, created_at, updated_at
		FROM virtual_keys WHERE id = ?`, id)
	vk, err := scanVirtualKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return VirtualKey{}, ErrNotFound
	}
	return vk, err
}

// UpsertVirtualKey inserts or replaces a row. Caller owns the id.
func (s *Store) UpsertVirtualKey(ctx context.Context, vk VirtualKey) error {
	if vk.ID == "" || vk.Name == "" {
		return errors.New("store: virtual_key.id and name are required")
	}
	models, _ := json.Marshal(nilSlice(vk.Models))
	ips, _ := json.Marshal(nilSlice(vk.IPAllowlist))
	var expires any
	if vk.ExpiresAt != nil {
		expires = vk.ExpiresAt.UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO virtual_keys (
			id, name, description, models, ip_allowlist,
			budget_tokens_daily, budget_tokens_monthly,
			budget_usd_daily, budget_usd_monthly, rpm_limit,
			enabled, expires_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name                  = excluded.name,
			description           = excluded.description,
			models                = excluded.models,
			ip_allowlist          = excluded.ip_allowlist,
			budget_tokens_daily   = excluded.budget_tokens_daily,
			budget_tokens_monthly = excluded.budget_tokens_monthly,
			budget_usd_daily      = excluded.budget_usd_daily,
			budget_usd_monthly    = excluded.budget_usd_monthly,
			rpm_limit             = excluded.rpm_limit,
			enabled               = excluded.enabled,
			expires_at            = excluded.expires_at,
			updated_at            = CURRENT_TIMESTAMP`,
		vk.ID, vk.Name, vk.Description, string(models), string(ips),
		vk.BudgetTokensDaily, vk.BudgetTokensMonthly,
		vk.BudgetUSDDaily, vk.BudgetUSDMonthly, vk.RPMLimit,
		boolToInt(vk.Enabled), expires)
	return err
}

// DeleteVirtualKey cascades to any linked usage_records rows via the FK.
func (s *Store) DeleteVirtualKey(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM virtual_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// -- usage --------------------------------------------------------------

// InsertUsage appends one usage row. Called from the request pipeline
// after a successful (or failed) upstream call.
func (s *Store) InsertUsage(ctx context.Context, u UsageRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO usage_records (
			virtual_key_id, ts, model, provider,
			prompt_tokens, completion_tokens, total_tokens,
			cost_usd, latency_ms, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.VirtualKeyID, u.Ts.UTC(), u.Model, u.Provider,
		u.PromptTokens, u.CompletionTokens, u.TotalTokens,
		u.CostUSD, u.LatencyMs, u.Status)
	return err
}

// SumUsage returns the aggregate totals for a virtual key over a time
// window. Both bounds are inclusive; pass a very old "from" and
// time.Now() to get "since ever".
func (s *Store) SumUsage(ctx context.Context, keyID string, from, to time.Time) (UsageTotals, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COUNT(*)
		FROM usage_records
		WHERE virtual_key_id = ? AND ts >= ? AND ts <= ?`,
		keyID, from.UTC(), to.UTC())
	var t UsageTotals
	if err := row.Scan(&t.Tokens, &t.PromptTokens, &t.CompletionTokens, &t.CostUSD, &t.Requests); err != nil {
		return UsageTotals{}, err
	}
	return t, nil
}

// RecentUsage returns the latest N usage rows for a virtual key, newest first.
// Pass an empty keyID to scan across all keys (admin dashboard view).
func (s *Store) RecentUsage(ctx context.Context, keyID string, limit int) ([]UsageRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if keyID == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, virtual_key_id, ts, model, provider,
			       prompt_tokens, completion_tokens, total_tokens,
			       cost_usd, latency_ms, status
			FROM usage_records ORDER BY ts DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, virtual_key_id, ts, model, provider,
			       prompt_tokens, completion_tokens, total_tokens,
			       cost_usd, latency_ms, status
			FROM usage_records WHERE virtual_key_id = ?
			ORDER BY ts DESC LIMIT ?`, keyID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRecord
	for rows.Next() {
		var u UsageRecord
		if err := rows.Scan(&u.ID, &u.VirtualKeyID, &u.Ts, &u.Model, &u.Provider,
			&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens,
			&u.CostUSD, &u.LatencyMs, &u.Status); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// -- scan helper --------------------------------------------------------

func scanVirtualKey(sc scanner) (VirtualKey, error) {
	var (
		vk                    VirtualKey
		models, ips           string
		enabledInt            int
		expiresAt             sql.NullTime
		createdAt, updatedAt  time.Time
	)
	if err := sc.Scan(
		&vk.ID, &vk.Name, &vk.Description, &models, &ips,
		&vk.BudgetTokensDaily, &vk.BudgetTokensMonthly,
		&vk.BudgetUSDDaily, &vk.BudgetUSDMonthly, &vk.RPMLimit,
		&enabledInt, &expiresAt, &createdAt, &updatedAt,
	); err != nil {
		return VirtualKey{}, fmt.Errorf("store: scan virtual_key: %w", err)
	}
	if models != "" {
		_ = json.Unmarshal([]byte(models), &vk.Models)
	}
	if ips != "" {
		_ = json.Unmarshal([]byte(ips), &vk.IPAllowlist)
	}
	vk.Enabled = enabledInt != 0
	if expiresAt.Valid {
		t := expiresAt.Time
		vk.ExpiresAt = &t
	}
	vk.CreatedAt = createdAt
	vk.UpdatedAt = updatedAt
	return vk, nil
}
