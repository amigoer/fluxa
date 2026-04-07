// regex_routes.go is the persistence layer for the v2.4 "intercept by
// pattern" feature. A regex route lets an operator say things like
// "any incoming model name matching ^gpt-4.* should be redirected to
// my qwen-latest virtual model" without touching the application code
// that ships those names. Patterns are evaluated in priority order
// (lower number = higher priority); the first match wins. The
// resolver in internal/router/model_resolver.go pre-compiles every
// pattern at reload time so the request path never pays a
// regexp.Compile cost.

package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

// RegexRoute mirrors one regex_routes row.
type RegexRoute struct {
	ID          string
	Pattern     string
	Priority    int
	TargetType  string // "real" | "virtual"
	TargetModel string
	Provider    string // required when TargetType == "real"
	Description string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListRegexRoutes returns every row sorted by priority ASC. Disabled
// rows are included so the dashboard can render them dimmed; the
// resolver filters on Enabled before evaluating.
func (s *Store) ListRegexRoutes(ctx context.Context) ([]RegexRoute, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, pattern, priority, target_type, target_model, provider,
		       description, enabled, created_at, updated_at
		FROM regex_routes ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RegexRoute
	for rows.Next() {
		var (
			r          RegexRoute
			enabledInt int
		)
		if err := rows.Scan(
			&r.ID, &r.Pattern, &r.Priority, &r.TargetType, &r.TargetModel,
			&r.Provider, &r.Description, &enabledInt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Enabled = enabledInt == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRegexRoute loads one row by id.
func (s *Store) GetRegexRoute(ctx context.Context, id string) (RegexRoute, error) {
	var (
		r          RegexRoute
		enabledInt int
	)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, pattern, priority, target_type, target_model, provider,
		       description, enabled, created_at, updated_at
		FROM regex_routes WHERE id = ?`, id)
	if err := row.Scan(
		&r.ID, &r.Pattern, &r.Priority, &r.TargetType, &r.TargetModel,
		&r.Provider, &r.Description, &enabledInt, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RegexRoute{}, ErrNotFound
		}
		return RegexRoute{}, err
	}
	r.Enabled = enabledInt == 1
	return r, nil
}

// validateRegexRoute is the shared validation used by both insert and
// update. We compile the pattern here even though the router does the
// same at reload time, because failing fast at the admin write path
// gives the operator an immediate 400 instead of a silent skip during
// the next reload (and potential surprise traffic redirection).
func validateRegexRoute(r *RegexRoute) error {
	if r.Pattern == "" {
		return errors.New("store: regex_route.pattern is required")
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return errors.New("store: regex_route.pattern does not compile: " + err.Error())
	}
	if r.TargetType != "real" && r.TargetType != "virtual" {
		return errors.New("store: regex_route.target_type must be 'real' or 'virtual'")
	}
	if r.TargetModel == "" {
		return errors.New("store: regex_route.target_model is required")
	}
	if r.TargetType == "real" && r.Provider == "" {
		return errors.New("store: regex_route.provider is required when target_type='real'")
	}
	return nil
}

// CreateRegexRoute inserts a new row, returning the persisted form
// (with id and timestamps populated by the database).
func (s *Store) CreateRegexRoute(ctx context.Context, r RegexRoute) (RegexRoute, error) {
	if err := validateRegexRoute(&r); err != nil {
		return RegexRoute{}, err
	}
	r.ID = newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO regex_routes (
			id, pattern, priority, target_type, target_model, provider,
			description, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		r.ID, r.Pattern, r.Priority, r.TargetType, r.TargetModel, r.Provider,
		r.Description, boolToInt(r.Enabled)); err != nil {
		return RegexRoute{}, err
	}
	return s.GetRegexRoute(ctx, r.ID)
}

// UpdateRegexRoute replaces every mutable field of an existing row.
func (s *Store) UpdateRegexRoute(ctx context.Context, r RegexRoute) (RegexRoute, error) {
	if r.ID == "" {
		return RegexRoute{}, errors.New("store: regex_route.id is required for update")
	}
	if err := validateRegexRoute(&r); err != nil {
		return RegexRoute{}, err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE regex_routes SET
			pattern      = ?,
			priority     = ?,
			target_type  = ?,
			target_model = ?,
			provider     = ?,
			description  = ?,
			enabled      = ?,
			updated_at   = CURRENT_TIMESTAMP
		WHERE id = ?`,
		r.Pattern, r.Priority, r.TargetType, r.TargetModel, r.Provider,
		r.Description, boolToInt(r.Enabled), r.ID)
	if err != nil {
		return RegexRoute{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return RegexRoute{}, ErrNotFound
	}
	return s.GetRegexRoute(ctx, r.ID)
}

// UpdateRegexRoutePriority is the narrow path used by drag-and-drop
// reordering in the dashboard. We expose it separately so the UI can
// reorder a list without having to round-trip every other field of
// the row (and risk a race with a concurrent edit).
func (s *Store) UpdateRegexRoutePriority(ctx context.Context, id string, priority int) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE regex_routes SET priority = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, priority, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRegexRoute removes one row by id.
func (s *Store) DeleteRegexRoute(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM regex_routes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
