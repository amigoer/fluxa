// analytics.go — time-bucketed rollups that back the console overview.
//
// These read request_logs rather than usage_records on purpose.
// usage_records is only written when an upstream call succeeds and
// returns a parseable usage block (see api.recordUsage), so a gateway
// having a bad day would show an empty dashboard exactly when the
// operator needs it most. request_logs has a row for every call —
// success, upstream error, and DLP block alike — which is what the
// overview is meant to reflect.

package store

import (
	"context"
	"time"
)

// UsageBucket is one day of traffic. Days with no traffic are still
// present with zero counts so the chart keeps a continuous x-axis.
type UsageBucket struct {
	Day      time.Time
	Requests int64
	Tokens   int64
	CostUSD  float64
	Errors   int64
}

// UsageBreakdown is one slice of a group-by, used for the provider and
// model rankings.
type UsageBreakdown struct {
	Name     string
	Requests int64
	Tokens   int64
	CostUSD  float64
}

// UsageTotalsWindow aggregates a whole time range. The overview renders
// two of these — the current window and the one before it — to show
// movement rather than a bare number.
type UsageTotalsWindow struct {
	Requests int64
	Tokens   int64
	CostUSD  float64
	Errors   int64
}

// UsageSeries returns one bucket per day in [from, to], zero-filled.
//
// generate_series does the zero-filling in the database, which keeps the
// "no traffic yet" case from becoming a special path in the frontend.
func (s *Store) UsageSeries(ctx context.Context, from, to time.Time) ([]UsageBucket, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.day,
		       COUNT(l.id),
		       COALESCE(SUM(l.total_tokens), 0),
		       COALESCE(SUM(l.cost_usd), 0),
		       COUNT(l.id) FILTER (WHERE l.status_code >= 400)
		FROM generate_series(
		         date_trunc('day', $1::timestamptz),
		         date_trunc('day', $2::timestamptz),
		         interval '1 day'
		     ) AS d(day)
		LEFT JOIN request_logs l
		       ON l.started_at >= d.day
		      AND l.started_at < d.day + interval '1 day'
		GROUP BY d.day
		ORDER BY d.day`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UsageBucket, 0, 32)
	for rows.Next() {
		var b UsageBucket
		if err := rows.Scan(&b.Day, &b.Requests, &b.Tokens, &b.CostUSD, &b.Errors); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UsageTotals sums a window. Kept separate from UsageSeries so the
// caller can ask for a previous period without pulling its buckets.
func (s *Store) UsageWindowTotals(ctx context.Context, from, to time.Time) (UsageTotalsWindow, error) {
	var t UsageTotalsWindow
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(total_tokens), 0),
		       COALESCE(SUM(cost_usd), 0),
		       COUNT(*) FILTER (WHERE status_code >= 400)
		FROM request_logs
		WHERE started_at >= $1 AND started_at <= $2`,
		from.UTC(), to.UTC()).Scan(&t.Requests, &t.Tokens, &t.CostUSD, &t.Errors)
	return t, err
}

// UsageByProvider ranks providers by request volume over the window.
// Rows written before a provider was resolved carry an empty string and
// are grouped under "unknown" rather than silently dropped.
func (s *Store) UsageByProvider(ctx context.Context, from, to time.Time, limit int) ([]UsageBreakdown, error) {
	return s.usageGroupBy(ctx, `COALESCE(NULLIF(provider, ''), 'unknown')`, from, to, limit)
}

// UsageByModel ranks models by request volume, preferring the resolved
// name so an alias and its target are not counted as two models.
func (s *Store) UsageByModel(ctx context.Context, from, to time.Time, limit int) ([]UsageBreakdown, error) {
	return s.usageGroupBy(ctx,
		`COALESCE(NULLIF(model_resolved, ''), NULLIF(model_requested, ''), 'unknown')`,
		from, to, limit)
}

// usageGroupBy is the shared body of the two rankings. The grouping
// expression is a trusted constant from this file, never user input.
func (s *Store) usageGroupBy(
	ctx context.Context,
	expr string,
	from, to time.Time,
	limit int,
) ([]UsageBreakdown, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+expr+` AS name,
		       COUNT(*),
		       COALESCE(SUM(total_tokens), 0),
		       COALESCE(SUM(cost_usd), 0)
		FROM request_logs
		WHERE started_at >= $1 AND started_at <= $2
		GROUP BY name
		ORDER BY COUNT(*) DESC, name
		LIMIT $3`, from.UTC(), to.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UsageBreakdown, 0, limit)
	for rows.Next() {
		var b UsageBreakdown
		if err := rows.Scan(&b.Name, &b.Requests, &b.Tokens, &b.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
