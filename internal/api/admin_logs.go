// admin_logs.go — admin REST endpoints for the request_logs table.
//
// Two surfaces:
//
//   GET /admin/logs       paginated list of recent calls. Returns a
//                         slim summary row per entry (no bodies) so a
//                         50-row page stays under a few hundred KiB
//                         even when the underlying log payloads are
//                         large.
//   GET /admin/logs/{id}  single row with full request and response
//                         bodies for debugging or replay.
//
// All query parameters are optional; the list endpoint degrades to a
// straight "most recent N rows" query when no filters are supplied.

package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// requestLogSummaryJSON is the slim list-row DTO. Body fields are
// intentionally absent — the operator inspects bodies on the detail
// endpoint after clicking a row.
type requestLogSummaryJSON struct {
	ID               string     `json:"id"`
	VirtualKeyID     string     `json:"virtual_key_id"`
	StartedAt        time.Time  `json:"started_at"`
	FirstByteAt      *time.Time `json:"first_byte_at,omitempty"`
	CompletedAt      time.Time  `json:"completed_at"`
	Endpoint         string     `json:"endpoint"`
	Method           string     `json:"method"`
	ModelRequested   string     `json:"model_requested"`
	ModelResolved    string     `json:"model_resolved"`
	Provider         string     `json:"provider"`
	IsStream         bool       `json:"is_stream"`
	CacheHit         bool       `json:"cache_hit"`
	StatusCode       int        `json:"status_code"`
	Error            string     `json:"error"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
	TotalTokens      int        `json:"total_tokens"`
	CostUSD          float64    `json:"cost_usd"`
	LatencyMs        int        `json:"latency_ms"`
	TTFTMs           int        `json:"ttft_ms"`
	ClientIP         string     `json:"client_ip"`
	UserAgent        string     `json:"user_agent"`
}

// requestLogDetailJSON is the full row. Bodies can be up to
// MaxRequestLogBodyBytes; the truncation marker tells the operator
// they are looking at a clipped payload.
type requestLogDetailJSON struct {
	requestLogSummaryJSON
	RequestBody  string `json:"request_body"`
	ResponseBody string `json:"response_body"`
}

func toSummaryJSON(l store.RequestLog) requestLogSummaryJSON {
	return requestLogSummaryJSON{
		ID:               l.ID,
		VirtualKeyID:     l.VirtualKeyID,
		StartedAt:        l.StartedAt,
		FirstByteAt:      l.FirstByteAt,
		CompletedAt:      l.CompletedAt,
		Endpoint:         l.Endpoint,
		Method:           l.Method,
		ModelRequested:   l.ModelRequested,
		ModelResolved:    l.ModelResolved,
		Provider:         l.Provider,
		IsStream:         l.IsStream,
		CacheHit:         l.CacheHit,
		StatusCode:       l.StatusCode,
		Error:            l.Error,
		PromptTokens:     l.PromptTokens,
		CompletionTokens: l.CompletionTokens,
		TotalTokens:      l.TotalTokens,
		CostUSD:          l.CostUSD,
		LatencyMs:        l.LatencyMs,
		TTFTMs:           l.TTFTMs,
		ClientIP:         l.ClientIP,
		UserAgent:        l.UserAgent,
	}
}

// listRequestLogs returns a paginated, filterable summary list. The
// response envelope carries `total` so the UI can render pagination
// without a second round trip.
func (a *AdminServer) listRequestLogs(w http.ResponseWriter, r *http.Request) {
	filter := parseRequestLogFilter(r)
	rows, total, err := a.store.ListRequestLogs(r.Context(), filter)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data := make([]requestLogSummaryJSON, 0, len(rows))
	for _, l := range rows {
		data = append(data, toSummaryJSON(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":   data,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// getRequestLog returns one row including full request and response
// bodies. 404 when the id is unknown.
func (a *AdminServer) getRequestLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "id is required")
		return
	}
	log, err := a.store.GetRequestLog(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "request log not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, requestLogDetailJSON{
		requestLogSummaryJSON: toSummaryJSON(log),
		RequestBody:           log.RequestBody,
		ResponseBody:          log.ResponseBody,
	})
}

// parseRequestLogFilter pulls a RequestLogFilter out of query
// parameters. Unknown / malformed values silently degrade to the
// zero value for that field; rejecting them would only ratchet
// the operator into reading the API docs for a query string typo.
func parseRequestLogFilter(r *http.Request) store.RequestLogFilter {
	q := r.URL.Query()
	f := store.RequestLogFilter{
		VirtualKeyID: q.Get("key_id"),
		Model:        q.Get("model"),
		Provider:     q.Get("provider"),
		Search:       q.Get("search"),
	}
	if v := q.Get("status_min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.StatusMin = n
		}
	}
	if v := q.Get("status_max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.StatusMax = n
		}
	}
	if v := q.Get("stream"); v != "" {
		switch v {
		case "true", "1":
			t := true
			f.Stream = &t
		case "false", "0":
			t := false
			f.Stream = &t
		}
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Until = t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	return f
}
