// admin_analytics.go serves the console overview in a single call.
//
// The dashboard needs four things at once — headline totals, a daily
// series, and two rankings — and issuing four requests for one screen
// makes the page render in stages. They are aggregated here instead so
// the overview paints once.

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// analyticsBucketJSON is one day of the trend line.
type analyticsBucketJSON struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Errors   int64   `json:"errors"`
}

type analyticsBreakdownJSON struct {
	Name     string  `json:"name"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

type analyticsTotalsJSON struct {
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Errors   int64   `json:"errors"`
}

type analyticsOverviewJSON struct {
	Days int `json:"days"`
	// Totals covers the requested window; Previous covers the window of
	// equal length immediately before it, so the UI can show movement.
	Totals     analyticsTotalsJSON      `json:"totals"`
	Previous   analyticsTotalsJSON      `json:"previous"`
	Series     []analyticsBucketJSON    `json:"series"`
	ByProvider []analyticsBreakdownJSON `json:"by_provider"`
	ByModel    []analyticsBreakdownJSON `json:"by_model"`
}

// maxOverviewDays caps the window so a mistyped query cannot ask the
// database to bucket several years of logs.
const maxOverviewDays = 90

// analyticsOverview returns the dashboard rollup for the last N days
// (default 7, capped at 90).
func (a *AdminServer) analyticsOverview(w http.ResponseWriter, r *http.Request) {
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = min(n, maxOverviewDays)
		}
	}

	now := time.Now().UTC()
	// Buckets are whole UTC days, so the window starts at midnight
	// days-1 ago: "7 days" means today plus the six before it.
	from := now.AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
	prevFrom := from.AddDate(0, 0, -days)
	prevTo := from.Add(-time.Nanosecond)

	ctx := r.Context()

	totals, err := a.store.UsageWindowTotals(ctx, from, now)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	previous, err := a.store.UsageWindowTotals(ctx, prevFrom, prevTo)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	buckets, err := a.store.UsageSeries(ctx, from, now)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byProvider, err := a.store.UsageByProvider(ctx, from, now, 8)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byModel, err := a.store.UsageByModel(ctx, from, now, 5)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := analyticsOverviewJSON{
		Days:       days,
		Totals:     analyticsTotalsJSON(totals),
		Previous:   analyticsTotalsJSON(previous),
		Series:     make([]analyticsBucketJSON, 0, len(buckets)),
		ByProvider: toBreakdownJSON(byProvider),
		ByModel:    toBreakdownJSON(byModel),
	}
	for _, b := range buckets {
		resp.Series = append(resp.Series, analyticsBucketJSON{
			Date:     b.Day.Format(time.DateOnly),
			Requests: b.Requests,
			Tokens:   b.Tokens,
			CostUSD:  b.CostUSD,
			Errors:   b.Errors,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func toBreakdownJSON(rows []store.UsageBreakdown) []analyticsBreakdownJSON {
	out := make([]analyticsBreakdownJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, analyticsBreakdownJSON(r))
	}
	return out
}
