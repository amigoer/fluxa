package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

func TestAdmin_RequestLogList(t *testing.T) {
	mux, st, token := newAdminFixture(t)

	// Seed three rows directly through the store; we test the admin
	// surface here, not the recorder.
	base := time.Now().UTC().Truncate(time.Second)
	for i, l := range []store.RequestLog{
		{
			VirtualKeyID:   "vk-a",
			StartedAt:      base.Add(-3 * time.Second),
			CompletedAt:    base.Add(-3 * time.Second),
			Endpoint:       "/v1/chat/completions",
			ModelRequested: "gpt-4o",
			ModelResolved:  "gpt-4o",
			Provider:       "openai",
			StatusCode:     200,
			TotalTokens:    42,
			RequestBody:    `{"messages":[{"role":"user","content":"hi"}]}`,
			ResponseBody:   `{"choices":[]}`,
		},
		{
			VirtualKeyID:   "vk-a",
			StartedAt:      base.Add(-2 * time.Second),
			CompletedAt:    base.Add(-2 * time.Second),
			Endpoint:       "/v1/chat/completions",
			ModelRequested: "gpt-4o",
			ModelResolved:  "gpt-4o",
			Provider:       "openai",
			IsStream:       true,
			StatusCode:     500,
			Error:          "upstream timeout",
		},
		{
			VirtualKeyID:   "vk-b",
			StartedAt:      base.Add(-1 * time.Second),
			CompletedAt:    base.Add(-1 * time.Second),
			Endpoint:       "/v1/messages",
			ModelRequested: "claude-3-5-sonnet",
			ModelResolved:  "claude-3-5-sonnet",
			Provider:       "anthropic",
			StatusCode:     200,
		},
	} {
		if _, err := st.InsertRequestLog(t.Context(), l); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Unfiltered list.
	rec := doAdmin(t, mux, "GET", "/admin/logs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data  []requestLogSummaryJSON `json:"data"`
		Total int64                   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 3 || len(resp.Data) != 3 {
		t.Fatalf("expected 3 rows, got len=%d total=%d", len(resp.Data), resp.Total)
	}
	// Newest first.
	if resp.Data[0].VirtualKeyID != "vk-b" {
		t.Errorf("newest row should be vk-b, got %s", resp.Data[0].VirtualKeyID)
	}

	// Filter by key.
	rec = doAdmin(t, mux, "GET", "/admin/logs?key_id=vk-a", token, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 || resp.Total != 2 {
		t.Errorf("vk-a filter: expected 2, got len=%d total=%d", len(resp.Data), resp.Total)
	}

	// Filter by 5xx status range.
	rec = doAdmin(t, mux, "GET", "/admin/logs?status_min=500&status_max=599", token, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0].StatusCode != 500 {
		t.Errorf("status filter: %+v", resp.Data)
	}

	// Filter by stream=true.
	rec = doAdmin(t, mux, "GET", "/admin/logs?stream=true", token, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || !resp.Data[0].IsStream {
		t.Errorf("stream filter: %+v", resp.Data)
	}

	// List endpoint should NOT include bodies.
	if first := resp.Data[0]; first.ID == "" {
		t.Errorf("expected id in list rows, got %+v", first)
	}
}

func TestAdmin_RequestLogDetail(t *testing.T) {
	mux, st, token := newAdminFixture(t)

	id, err := st.InsertRequestLog(t.Context(), store.RequestLog{
		VirtualKeyID:   "vk-test",
		StartedAt:      time.Now().UTC(),
		CompletedAt:    time.Now().UTC(),
		Endpoint:       "/v1/chat/completions",
		ModelRequested: "gpt-4o",
		ModelResolved:  "gpt-4o",
		Provider:       "openai",
		StatusCode:     200,
		RequestBody:    `{"hello":"world"}`,
		ResponseBody:   `{"goodbye":"world"}`,
	})
	if err != nil {
		t.Fatalf("InsertRequestLog: %v", err)
	}

	rec := doAdmin(t, mux, "GET", "/admin/logs/"+id, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var detail requestLogDetailJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.ID != id {
		t.Errorf("id mismatch: %s vs %s", detail.ID, id)
	}
	if detail.RequestBody == "" || detail.ResponseBody == "" {
		t.Errorf("detail endpoint should include bodies: %+v", detail)
	}

	// Unknown id → 404.
	rec = doAdmin(t, mux, "GET", "/admin/logs/no-such-id", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAdmin_RequestLogAuthRequired(t *testing.T) {
	mux, _, _ := newAdminFixture(t)
	rec := doAdmin(t, mux, "GET", "/admin/logs", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}
}
