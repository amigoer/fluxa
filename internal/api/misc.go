package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// handleListModels implements GET /v1/models. The response shape mirrors
// OpenAI's so existing SDKs that populate model pickers work unchanged.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	ids := s.router.Models()
	sort.Strings(ids)
	out := struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}{Object: "list"}
	for _, id := range ids {
		out.Data = append(out.Data, model{ID: id, Object: "model", OwnedBy: "fluxa"})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleHealth implements GET /health. It reports both the gateway's own
// liveness and the reachability of every configured upstream provider. The
// endpoint returns 200 only when every provider is healthy so it can be used
// directly as a Kubernetes readiness probe.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	reports := s.router.CheckHealth(ctx)
	allOK := true
	for _, r := range reports {
		if !r.OK {
			allOK = false
			break
		}
	}

	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    boolToStatus(allOK),
		"providers": reports,
	})
}

func boolToStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}
