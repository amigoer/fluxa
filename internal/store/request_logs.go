// request_logs.go — persistence of raw LLM request/response logs.
//
// One row is appended per /v1/chat/completions or /v1/messages call,
// regardless of outcome. The row carries enough payload to reconstruct
// what the upstream provider saw (full request body) and what the
// client received (full response body, accumulated across SSE chunks
// for streaming responses), so operators can debug routing decisions,
// replay calls, and audit what data left the network.
//
// usage_records remains the authoritative source of aggregated
// metrics (budgets, dashboards). request_logs is the raw stream and
// is expected to be pruned by a retention job in a future iteration.

package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// RequestLog mirrors one request_logs row.
type RequestLog struct {
	ID               string
	VirtualKeyID     string
	StartedAt        time.Time
	FirstByteAt      *time.Time // nil unless the response was streamed
	CompletedAt      time.Time
	Endpoint         string // e.g. /v1/chat/completions
	Method           string
	ModelRequested   string // model field as it arrived from the client
	ModelResolved    string // model name actually sent to the upstream
	Provider         string
	IsStream         bool
	CacheHit         bool // reserved — wired in by a future cache layer
	StatusCode       int
	Error            string // empty on success, otherwise short message
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostUSD          float64
	LatencyMs        int
	TTFTMs           int // 0 for non-streaming calls or when unknown
	RequestBody      string
	ResponseBody     string
	ClientIP         string
	UserAgent        string
}

// RequestLogFilter narrows a ListRequestLogs query. Zero values mean
// "no constraint on this dimension". Stream is a pointer so callers
// can distinguish "false" (only non-stream) from "unset" (either).
type RequestLogFilter struct {
	VirtualKeyID string
	Model        string // matches ModelRequested OR ModelResolved
	Provider     string
	StatusMin    int // inclusive; 0 = no lower bound
	StatusMax    int // inclusive; 0 = no upper bound
	Stream       *bool
	Since        time.Time
	Until        time.Time
	Search       string // free-text match against bodies and error
	Limit        int    // page size, default 50, capped at 500
	Offset       int
}

// InsertRequestLog appends a single row. Callers fill in the
// timing/usage fields; the store only assigns an ID when one is not
// already set, and defaults missing timestamps so partial records
// (e.g. failures before completion) still land.
func (s *Store) InsertRequestLog(ctx context.Context, l RequestLog) (string, error) {
	if l.ID == "" {
		l.ID = newID()
	}
	if l.StartedAt.IsZero() {
		l.StartedAt = time.Now()
	}
	if l.CompletedAt.IsZero() {
		l.CompletedAt = l.StartedAt
	}
	var firstByteAt sql.NullTime
	if l.FirstByteAt != nil {
		firstByteAt = sql.NullTime{Time: *l.FirstByteAt, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO request_logs (
			id, virtual_key_id, started_at, first_byte_at, completed_at,
			endpoint, method, model_requested, model_resolved, provider,
			is_stream, cache_hit, status_code, error,
			prompt_tokens, completion_tokens, total_tokens, cost_usd,
			latency_ms, ttft_ms, request_body, response_body,
			client_ip, user_agent
		) VALUES (?, ?, ?, ?, ?,
		          ?, ?, ?, ?, ?,
		          ?, ?, ?, ?,
		          ?, ?, ?, ?,
		          ?, ?, ?, ?,
		          ?, ?)`,
		l.ID, l.VirtualKeyID, l.StartedAt, firstByteAt, l.CompletedAt,
		l.Endpoint, l.Method, l.ModelRequested, l.ModelResolved, l.Provider,
		boolToInt(l.IsStream), boolToInt(l.CacheHit), l.StatusCode, l.Error,
		l.PromptTokens, l.CompletionTokens, l.TotalTokens, l.CostUSD,
		l.LatencyMs, l.TTFTMs, l.RequestBody, l.ResponseBody,
		l.ClientIP, l.UserAgent,
	)
	if err != nil {
		return "", err
	}
	return l.ID, nil
}

// GetRequestLog loads one row by id. Returns ErrNotFound when missing.
func (s *Store) GetRequestLog(ctx context.Context, id string) (RequestLog, error) {
	row := s.db.QueryRowContext(ctx, requestLogSelectCols+` WHERE id = ?`, id)
	l, err := scanRequestLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RequestLog{}, ErrNotFound
	}
	return l, err
}

// ListRequestLogs returns matching rows in reverse-chronological order
// with pagination. The second return value is the total count for the
// same filter (ignoring limit/offset) so the UI can render pagination
// without a second round trip.
func (s *Store) ListRequestLogs(ctx context.Context, f RequestLogFilter) ([]RequestLog, int64, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	where, args := buildRequestLogWhere(f)

	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM request_logs `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx,
		requestLogSelectCols+` `+where+` ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?`,
		pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []RequestLog
	for rows.Next() {
		l, err := scanRequestLog(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// requestLogSelectCols is the canonical column list shared by the
// single-row Get and paginated List queries so the two scanners stay
// in lockstep. Kept as a string constant rather than a stringer so
// query construction remains a plain concatenation.
const requestLogSelectCols = `
	SELECT id, virtual_key_id, started_at, first_byte_at, completed_at,
	       endpoint, method, model_requested, model_resolved, provider,
	       is_stream, cache_hit, status_code, error,
	       prompt_tokens, completion_tokens, total_tokens, cost_usd,
	       latency_ms, ttft_ms, request_body, response_body,
	       client_ip, user_agent
	FROM request_logs`

// buildRequestLogWhere assembles the WHERE clause from a filter.
// Returns "" + nil when the filter is empty so the query degrades to
// a full scan against the started_at index.
func buildRequestLogWhere(f RequestLogFilter) (string, []any) {
	var (
		clauses []string
		args    []any
	)
	if f.VirtualKeyID != "" {
		clauses = append(clauses, "virtual_key_id = ?")
		args = append(args, f.VirtualKeyID)
	}
	if f.Model != "" {
		clauses = append(clauses, "(model_requested = ? OR model_resolved = ?)")
		args = append(args, f.Model, f.Model)
	}
	if f.Provider != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.StatusMin > 0 {
		clauses = append(clauses, "status_code >= ?")
		args = append(args, f.StatusMin)
	}
	if f.StatusMax > 0 {
		clauses = append(clauses, "status_code <= ?")
		args = append(args, f.StatusMax)
	}
	if f.Stream != nil {
		clauses = append(clauses, "is_stream = ?")
		args = append(args, boolToInt(*f.Stream))
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "started_at >= ?")
		args = append(args, f.Since)
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "started_at <= ?")
		args = append(args, f.Until)
	}
	if f.Search != "" {
		clauses = append(clauses, "(request_body LIKE ? OR response_body LIKE ? OR error LIKE ?)")
		needle := "%" + f.Search + "%"
		args = append(args, needle, needle, needle)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// scanRequestLog reads one row into RequestLog. Works with both
// *sql.Row and *sql.Rows via the package-level scanner interface.
func scanRequestLog(sc scanner) (RequestLog, error) {
	var (
		l                   RequestLog
		streamInt, cacheInt int
		firstByteAt         sql.NullTime
	)
	if err := sc.Scan(
		&l.ID, &l.VirtualKeyID, &l.StartedAt, &firstByteAt, &l.CompletedAt,
		&l.Endpoint, &l.Method, &l.ModelRequested, &l.ModelResolved, &l.Provider,
		&streamInt, &cacheInt, &l.StatusCode, &l.Error,
		&l.PromptTokens, &l.CompletionTokens, &l.TotalTokens, &l.CostUSD,
		&l.LatencyMs, &l.TTFTMs, &l.RequestBody, &l.ResponseBody,
		&l.ClientIP, &l.UserAgent,
	); err != nil {
		return RequestLog{}, err
	}
	l.IsStream = streamInt != 0
	l.CacheHit = cacheInt != 0
	if firstByteAt.Valid {
		t := firstByteAt.Time
		l.FirstByteAt = &t
	}
	return l, nil
}
