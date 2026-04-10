// admin_dlp.go exposes the DLP rule management endpoints for the
// admin dashboard. Follows the same pattern as admin_resolver.go:
// thin handlers that decode JSON, call the store, then reload the
// in-memory engine so the data plane picks up changes immediately.

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// -- wire types ---------------------------------------------------------

type dlpRuleDTO struct {
	ID           string    `json:"id,omitempty"`
	Name         string    `json:"name"`
	Pattern      string    `json:"pattern"`
	PatternType  string    `json:"pattern_type"`
	Scope        string    `json:"scope"`
	Action       string    `json:"action"`
	Priority     int       `json:"priority"`
	ModelPattern string    `json:"model_pattern,omitempty"`
	Description  string    `json:"description,omitempty"`
	Enabled      *bool     `json:"enabled,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type dlpViolationDTO struct {
	ID          int64     `json:"id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	KeyID       string    `json:"key_id"`
	Model       string    `json:"model"`
	Direction   string    `json:"direction"`
	MatchedText string    `json:"matched_text"`
	ActionTaken string    `json:"action_taken"`
	CreatedAt   time.Time `json:"created_at"`
}

func toDLPRuleDTO(r store.DLPRule) dlpRuleDTO {
	enabled := r.Enabled
	return dlpRuleDTO{
		ID:           r.ID,
		Name:         r.Name,
		Pattern:      r.Pattern,
		PatternType:  r.PatternType,
		Scope:        r.Scope,
		Action:       r.Action,
		Priority:     r.Priority,
		ModelPattern: r.ModelPattern,
		Description:  r.Description,
		Enabled:      &enabled,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func fromDLPRuleDTO(dto dlpRuleDTO) store.DLPRule {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	return store.DLPRule{
		ID:           dto.ID,
		Name:         dto.Name,
		Pattern:      dto.Pattern,
		PatternType:  dto.PatternType,
		Scope:        dto.Scope,
		Action:       dto.Action,
		Priority:     dto.Priority,
		ModelPattern: dto.ModelPattern,
		Description:  dto.Description,
		Enabled:      enabled,
	}
}

func toDLPViolationDTO(v store.DLPViolation) dlpViolationDTO {
	return dlpViolationDTO{
		ID:          v.ID,
		RuleID:      v.RuleID,
		RuleName:    v.RuleName,
		KeyID:       v.KeyID,
		Model:       v.Model,
		Direction:   v.Direction,
		MatchedText: v.MatchedText,
		ActionTaken: v.ActionTaken,
		CreatedAt:   v.CreatedAt,
	}
}

// -- rule handlers ------------------------------------------------------

func (a *AdminServer) listDLPRules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListDLPRules(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]dlpRuleDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDLPRuleDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getDLPRule(w http.ResponseWriter, r *http.Request) {
	row, err := a.store.GetDLPRule(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "dlp rule not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDLPRuleDTO(row))
}

func (a *AdminServer) createDLPRule(w http.ResponseWriter, r *http.Request) {
	var dto dlpRuleDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	saved, err := a.store.CreateDLPRule(r.Context(), fromDLPRuleDTO(dto))
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.dlpEngine != nil {
		if err := a.dlpEngine.Reload(r.Context()); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
			return
		}
	}
	a.logger.Info("admin dlp rule created", "id", saved.ID, "name", saved.Name)
	writeJSON(w, http.StatusOK, toDLPRuleDTO(saved))
}

func (a *AdminServer) updateDLPRule(w http.ResponseWriter, r *http.Request) {
	var dto dlpRuleDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	dto.ID = r.PathValue("id")
	saved, err := a.store.UpdateDLPRule(r.Context(), fromDLPRuleDTO(dto))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "dlp rule not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.dlpEngine != nil {
		if err := a.dlpEngine.Reload(r.Context()); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, toDLPRuleDTO(saved))
}

func (a *AdminServer) updateDLPRulePriority(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	id := r.PathValue("id")
	if err := a.store.UpdateDLPRulePriority(r.Context(), id, body.Priority); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "dlp rule not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a.dlpEngine != nil {
		if err := a.dlpEngine.Reload(r.Context()); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminServer) deleteDLPRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteDLPRule(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "dlp rule not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a.dlpEngine != nil {
		if err := a.dlpEngine.Reload(r.Context()); err != nil {
			a.logger.Warn("router reload after dlp rule delete failed", "id", id, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- violation handlers -------------------------------------------------

func (a *AdminServer) listDLPViolations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	ruleID := q.Get("rule_id")

	rows, total, err := a.store.ListDLPViolations(r.Context(), limit, offset, ruleID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]dlpViolationDTO, 0, len(rows))
	for _, v := range rows {
		out = append(out, toDLPViolationDTO(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "total": total})
}
