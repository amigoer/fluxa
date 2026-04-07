// admin_stats.go — handler stub for the live-mode edge stats endpoint
// consumed by the visual route graph editor in the admin dashboard.
//
// The dashboard polls GET /admin/stats/edges every 3 seconds while live
// mode is on and expects a payload of the shape:
//
//	{ "data": { "<edge-id>": { "rps": 12.4, "errorRate": 0.012 }, ... } }
//
// where edge-id matches the React Flow edge ids that buildGraph.ts
// emits client-side. The frontend already has a mock fallback for when
// this endpoint returns nothing, so we ship the route as a no-op stub
// today and wire real metrics from the request log later — the
// contract is fixed, only the body changes when the implementation
// arrives.

package api

import (
	"encoding/json"
	"net/http"
)

// EdgeStat is a single live sample for one edge in the routing graph.
// rps is requests-per-second over a rolling window (the dashboard
// expects ~30 s); errorRate is a 0..1 fraction of failures inside the
// same window. Field names are lowerCamelCase to match what the React
// Flow client deserialises directly into its store.
type EdgeStat struct {
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"errorRate"`
}

// edgeStatsResponse wraps the per-edge map in the same { data: ... }
// envelope every other admin list endpoint uses, so the frontend's
// thin fetch wrapper does not need a special case.
type edgeStatsResponse struct {
	Data map[string]EdgeStat `json:"data"`
}

// statsEdges is the GET /admin/stats/edges handler. Today it returns
// an empty map; tomorrow it will aggregate request log rows by
// (regex_id | virtual_model_route_id) and emit one EdgeStat per edge
// id the dashboard knows about.
func (a *AdminServer) statsEdges(w http.ResponseWriter, _ *http.Request) {
	resp := edgeStatsResponse{Data: map[string]EdgeStat{}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
