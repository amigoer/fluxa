// admin_keys.go — REST endpoints for virtual key CRUD and usage
// reporting. Every mutation writes through the store and then reloads
// the in-memory Keyring so policy changes take effect immediately
// without bouncing the process.
//
// Wire format mirrors the OpenAI admin-style envelope: list endpoints
// return {"data": [...]}, single-object endpoints return the bare row.
// Errors use the same writeAdminError helper as the provider/route
// handlers.

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/amigoer/fluxa/internal/keys"
	"github.com/amigoer/fluxa/internal/store"
)

// virtualKeyDTO is the JSON shape for a virtual key. Budget fields are
// surfaced as plain numbers; zero means "unlimited" so operators can
// freely omit them. ExpiresAt is RFC3339 when set, omitted otherwise.
// On create the server fills Id itself — clients never pick their own.
type virtualKeyDTO struct {
	ID                  string     `json:"id,omitempty"`
	Name                string     `json:"name"`
	Description         string     `json:"description,omitempty"`
	Models              []string   `json:"models,omitempty"`
	IPAllowlist         []string   `json:"ip_allowlist,omitempty"`
	BudgetTokensDaily   int64      `json:"budget_tokens_daily,omitempty"`
	BudgetTokensMonthly int64      `json:"budget_tokens_monthly,omitempty"`
	BudgetUSDDaily      float64    `json:"budget_usd_daily,omitempty"`
	BudgetUSDMonthly    float64    `json:"budget_usd_monthly,omitempty"`
	RPMLimit            int        `json:"rpm_limit,omitempty"`
	Enabled             *bool      `json:"enabled,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at,omitempty"`
}

func toKeyDTO(vk store.VirtualKey) virtualKeyDTO {
	enabled := vk.Enabled
	return virtualKeyDTO{
		ID:                  vk.ID,
		Name:                vk.Name,
		Description:         vk.Description,
		Models:              vk.Models,
		IPAllowlist:         vk.IPAllowlist,
		BudgetTokensDaily:   vk.BudgetTokensDaily,
		BudgetTokensMonthly: vk.BudgetTokensMonthly,
		BudgetUSDDaily:      vk.BudgetUSDDaily,
		BudgetUSDMonthly:    vk.BudgetUSDMonthly,
		RPMLimit:            vk.RPMLimit,
		Enabled:             &enabled,
		ExpiresAt:           vk.ExpiresAt,
		CreatedAt:           vk.CreatedAt,
		UpdatedAt:           vk.UpdatedAt,
	}
}

func fromKeyDTO(dto virtualKeyDTO) store.VirtualKey {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	return store.VirtualKey{
		ID:                  dto.ID,
		Name:                dto.Name,
		Description:         dto.Description,
		Models:              dto.Models,
		IPAllowlist:         dto.IPAllowlist,
		BudgetTokensDaily:   dto.BudgetTokensDaily,
		BudgetTokensMonthly: dto.BudgetTokensMonthly,
		BudgetUSDDaily:      dto.BudgetUSDDaily,
		BudgetUSDMonthly:    dto.BudgetUSDMonthly,
		RPMLimit:            dto.RPMLimit,
		Enabled:             enabled,
		ExpiresAt:           dto.ExpiresAt,
	}
}

// reloadKeyring refreshes the in-memory ring if one is attached. Writes
// still succeed when no ring exists (CLI / headless mode) because the
// next process start will re-read the store.
func (a *AdminServer) reloadKeyring(r *http.Request) {
	if a.keyring == nil {
		return
	}
	if err := a.keyring.Reload(r.Context()); err != nil {
		a.logger.Warn("keyring reload failed", "err", err)
	}
}

// -- list / get ---------------------------------------------------------

func (a *AdminServer) listKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListVirtualKeys(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]virtualKeyDTO, 0, len(rows))
	for _, vk := range rows {
		out = append(out, toKeyDTO(vk))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getKey(w http.ResponseWriter, r *http.Request) {
	vk, err := a.store.GetVirtualKey(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "virtual key not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toKeyDTO(vk))
}

// -- create / update ----------------------------------------------------

func (a *AdminServer) createKey(w http.ResponseWriter, r *http.Request) {
	var dto virtualKeyDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if dto.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "name is required")
		return
	}
	// Server owns the id so the "vk-" prefix invariant stays sacred.
	id, err := keys.GenerateID()
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto.ID = id
	if dto.Enabled == nil {
		t := true
		dto.Enabled = &t
	}
	if err := a.store.UpsertVirtualKey(r.Context(), fromKeyDTO(dto)); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.reloadKeyring(r)
	out, _ := a.store.GetVirtualKey(r.Context(), id)
	a.logger.Info("admin virtual key created", "id", id, "name", out.Name)
	writeJSON(w, http.StatusCreated, toKeyDTO(out))
}

func (a *AdminServer) updateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := a.store.GetVirtualKey(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "virtual key not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Start from the existing row so callers can send a partial patch.
	dto := toKeyDTO(existing)
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	dto.ID = id // path id always wins
	if err := a.store.UpsertVirtualKey(r.Context(), fromKeyDTO(dto)); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.reloadKeyring(r)
	out, _ := a.store.GetVirtualKey(r.Context(), id)
	a.logger.Info("admin virtual key updated", "id", id)
	writeJSON(w, http.StatusOK, toKeyDTO(out))
}

func (a *AdminServer) deleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteVirtualKey(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "virtual key not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.reloadKeyring(r)
	a.logger.Info("admin virtual key deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// -- usage --------------------------------------------------------------

// listUsage returns the most recent usage rows, optionally filtered by
// ?key_id=. Limit defaults to 100 and is clamped to 1000 so a typo
// cannot accidentally scan the whole table.
func (a *AdminServer) listUsage(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := a.store.RecentUsage(r.Context(), r.URL.Query().Get("key_id"), limit)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}

// usageSummary returns day / month totals for a single key, or across
// every key when ?key_id is omitted. The response is what the frontend
// dashboard renders at the top of the usage page.
func (a *AdminServer) usageSummary(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("key_id")
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	daily, err := a.sumUsageForSummary(r, keyID, dayStart, now)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	monthly, err := a.sumUsageForSummary(r, keyID, monthStart, now)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key_id":  keyID,
		"daily":   daily,
		"monthly": monthly,
	})
}

// sumUsageForSummary collapses the key-specific vs all-keys code paths
// into one helper so the JSON-producing handler stays linear.
func (a *AdminServer) sumUsageForSummary(r *http.Request, keyID string, from, to time.Time) (store.UsageTotals, error) {
	if keyID != "" {
		return a.store.SumUsage(r.Context(), keyID, from, to)
	}
	// Aggregate over every key by summing per-key rows. It is cheaper
	// than a dedicated query because the list is small (operators
	// usually have tens of keys, not millions) and avoids another SQL
	// variant on the store.
	rows, err := a.store.ListVirtualKeys(r.Context())
	if err != nil {
		return store.UsageTotals{}, err
	}
	var total store.UsageTotals
	for _, vk := range rows {
		t, err := a.store.SumUsage(r.Context(), vk.ID, from, to)
		if err != nil {
			return store.UsageTotals{}, err
		}
		total.Tokens += t.Tokens
		total.PromptTokens += t.PromptTokens
		total.CompletionTokens += t.CompletionTokens
		total.CostUSD += t.CostUSD
		total.Requests += t.Requests
	}
	return total, nil
}
