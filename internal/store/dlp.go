// dlp.go is the persistence layer for DLP (Data Loss Prevention) rules
// and their violation audit log. Rules define patterns that are matched
// against request/response content; violations record every match for
// compliance review.

package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

// DLPRule mirrors one dlp_rules row.
type DLPRule struct {
	ID           string
	Name         string
	Pattern      string
	PatternType  string // "keyword" | "regex"
	Scope        string // "request" | "response" | "both"
	Action       string // "block" | "mask" | "log"
	Priority     int
	ModelPattern string // optional regex scoping to specific models
	Description  string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DLPViolation mirrors one dlp_violations row.
type DLPViolation struct {
	ID          int64
	RuleID      string
	RuleName    string
	KeyID       string
	Model       string
	Direction   string // "request" | "response"
	MatchedText string
	ActionTaken string
	CreatedAt   time.Time
}

// ListDLPRules returns every rule sorted by priority ASC. Disabled
// rows are included so the dashboard can render them dimmed.
func (s *Store) ListDLPRules(ctx context.Context) ([]DLPRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, pattern, pattern_type, scope, action, priority,
		       model_pattern, description, enabled, created_at, updated_at
		FROM dlp_rules ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DLPRule
	for rows.Next() {
		var (
			r          DLPRule
			enabledInt int
		)
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Pattern, &r.PatternType, &r.Scope, &r.Action,
			&r.Priority, &r.ModelPattern, &r.Description, &enabledInt,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Enabled = enabledInt == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDLPRule loads one row by id.
func (s *Store) GetDLPRule(ctx context.Context, id string) (DLPRule, error) {
	var (
		r          DLPRule
		enabledInt int
	)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, pattern, pattern_type, scope, action, priority,
		       model_pattern, description, enabled, created_at, updated_at
		FROM dlp_rules WHERE id = ?`, id)
	if err := row.Scan(
		&r.ID, &r.Name, &r.Pattern, &r.PatternType, &r.Scope, &r.Action,
		&r.Priority, &r.ModelPattern, &r.Description, &enabledInt,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DLPRule{}, ErrNotFound
		}
		return DLPRule{}, err
	}
	r.Enabled = enabledInt == 1
	return r, nil
}

// validateDLPRule checks the rule for structural correctness before
// writing. Regex patterns are compiled to fail fast at the admin
// write path rather than silently failing at scan time.
func validateDLPRule(r *DLPRule) error {
	if r.Name == "" {
		return errors.New("store: dlp_rule.name is required")
	}
	if r.Pattern == "" {
		return errors.New("store: dlp_rule.pattern is required")
	}
	if r.PatternType != "keyword" && r.PatternType != "regex" {
		return errors.New("store: dlp_rule.pattern_type must be 'keyword' or 'regex'")
	}
	if r.PatternType == "regex" {
		if _, err := regexp.Compile(r.Pattern); err != nil {
			return errors.New("store: dlp_rule.pattern does not compile: " + err.Error())
		}
	}
	if r.Scope != "request" && r.Scope != "response" && r.Scope != "both" {
		return errors.New("store: dlp_rule.scope must be 'request', 'response', or 'both'")
	}
	if r.Action != "block" && r.Action != "mask" && r.Action != "log" {
		return errors.New("store: dlp_rule.action must be 'block', 'mask', or 'log'")
	}
	if r.ModelPattern != "" {
		if _, err := regexp.Compile(r.ModelPattern); err != nil {
			return errors.New("store: dlp_rule.model_pattern does not compile: " + err.Error())
		}
	}
	return nil
}

// CreateDLPRule inserts a new row, returning the persisted form.
func (s *Store) CreateDLPRule(ctx context.Context, r DLPRule) (DLPRule, error) {
	if err := validateDLPRule(&r); err != nil {
		return DLPRule{}, err
	}
	r.ID = newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO dlp_rules (
			id, name, pattern, pattern_type, scope, action, priority,
			model_pattern, description, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		r.ID, r.Name, r.Pattern, r.PatternType, r.Scope, r.Action,
		r.Priority, r.ModelPattern, r.Description, boolToInt(r.Enabled)); err != nil {
		return DLPRule{}, err
	}
	return s.GetDLPRule(ctx, r.ID)
}

// UpdateDLPRule replaces every mutable field of an existing row.
func (s *Store) UpdateDLPRule(ctx context.Context, r DLPRule) (DLPRule, error) {
	if r.ID == "" {
		return DLPRule{}, errors.New("store: dlp_rule.id is required for update")
	}
	if err := validateDLPRule(&r); err != nil {
		return DLPRule{}, err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE dlp_rules SET
			name          = ?,
			pattern       = ?,
			pattern_type  = ?,
			scope         = ?,
			action        = ?,
			priority      = ?,
			model_pattern = ?,
			description   = ?,
			enabled       = ?,
			updated_at    = CURRENT_TIMESTAMP
		WHERE id = ?`,
		r.Name, r.Pattern, r.PatternType, r.Scope, r.Action,
		r.Priority, r.ModelPattern, r.Description, boolToInt(r.Enabled), r.ID)
	if err != nil {
		return DLPRule{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return DLPRule{}, ErrNotFound
	}
	return s.GetDLPRule(ctx, r.ID)
}

// UpdateDLPRulePriority is the narrow path for drag-and-drop reordering.
func (s *Store) UpdateDLPRulePriority(ctx context.Context, id string, priority int) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE dlp_rules SET priority = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, priority, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDLPRule removes one row by id.
func (s *Store) DeleteDLPRule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM dlp_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// InsertDLPViolation appends one violation record. This is append-only;
// there is no update or delete path.
func (s *Store) InsertDLPViolation(ctx context.Context, v DLPViolation) error {
	// Truncate matched text to prevent the audit log from becoming
	// a data leak vector itself.
	text := v.MatchedText
	if len(text) > 200 {
		text = text[:200]
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dlp_violations (
			rule_id, rule_name, key_id, model, direction,
			matched_text, action_taken, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		v.RuleID, v.RuleName, v.KeyID, v.Model, v.Direction,
		text, v.ActionTaken)
	return err
}

// ListDLPViolations returns violations in reverse chronological order
// with pagination. An empty ruleID means no filter.
func (s *Store) ListDLPViolations(ctx context.Context, limit, offset int, ruleID string) ([]DLPViolation, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Count total for pagination.
	var total int
	if ruleID != "" {
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM dlp_violations WHERE rule_id = ?`, ruleID).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
	} else {
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM dlp_violations`).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
	}

	// Fetch page.
	var (
		rows *sql.Rows
		err  error
	)
	if ruleID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, rule_id, rule_name, key_id, model, direction,
			       matched_text, action_taken, created_at
			FROM dlp_violations WHERE rule_id = ?
			ORDER BY created_at DESC LIMIT ? OFFSET ?`, ruleID, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, rule_id, rule_name, key_id, model, direction,
			       matched_text, action_taken, created_at
			FROM dlp_violations
			ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []DLPViolation
	for rows.Next() {
		var v DLPViolation
		if err := rows.Scan(
			&v.ID, &v.RuleID, &v.RuleName, &v.KeyID, &v.Model,
			&v.Direction, &v.MatchedText, &v.ActionTaken, &v.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
