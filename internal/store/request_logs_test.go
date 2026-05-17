package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRequestLogCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	firstByte := now.Add(200 * time.Millisecond)

	id, err := s.InsertRequestLog(ctx, RequestLog{
		VirtualKeyID:     "vk-test",
		StartedAt:        now,
		FirstByteAt:      &firstByte,
		CompletedAt:      now.Add(time.Second),
		Endpoint:         "/v1/chat/completions",
		Method:           "POST",
		ModelRequested:   "gpt-4o",
		ModelResolved:    "gpt-4o-2024",
		Provider:         "openai",
		IsStream:         true,
		StatusCode:       200,
		PromptTokens:     12,
		CompletionTokens: 34,
		TotalTokens:      46,
		CostUSD:          0.0012,
		LatencyMs:        1000,
		TTFTMs:           200,
		RequestBody:      `{"messages":[{"role":"user","content":"hi"}]}`,
		ResponseBody:     `data: {"choices":[{"delta":{"content":"hello"}}]}`,
		ClientIP:         "10.0.0.1",
		UserAgent:        "fluxa-test",
	})
	if err != nil {
		t.Fatalf("InsertRequestLog: %v", err)
	}
	if id == "" {
		t.Fatal("expected id to be assigned")
	}

	got, err := s.GetRequestLog(ctx, id)
	if err != nil {
		t.Fatalf("GetRequestLog: %v", err)
	}
	if got.ID != id {
		t.Errorf("id mismatch: %s vs %s", got.ID, id)
	}
	if !got.IsStream {
		t.Error("is_stream lost")
	}
	if got.FirstByteAt == nil || !got.FirstByteAt.Equal(firstByte) {
		t.Errorf("first_byte_at mismatch: got %v want %v", got.FirstByteAt, firstByte)
	}
	if got.TTFTMs != 200 || got.LatencyMs != 1000 {
		t.Errorf("timing mismatch: ttft=%d lat=%d", got.TTFTMs, got.LatencyMs)
	}
	if got.PromptTokens != 12 || got.CompletionTokens != 34 || got.TotalTokens != 46 {
		t.Errorf("tokens mismatch: %+v", got)
	}
	if got.RequestBody == "" || got.ResponseBody == "" {
		t.Error("bodies should be persisted")
	}
}

func TestRequestLogListAndFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Seed three rows: two for vk-a (one stream, one non-stream), one
	// for vk-b. Spread the timestamps so ORDER BY started_at DESC
	// surfaces a predictable ordering.
	base := time.Now().UTC().Truncate(time.Second)
	rows := []RequestLog{
		{
			VirtualKeyID:   "vk-a",
			StartedAt:      base.Add(-3 * time.Second),
			CompletedAt:    base.Add(-3 * time.Second),
			Endpoint:       "/v1/chat/completions",
			ModelRequested: "gpt-4o",
			ModelResolved:  "gpt-4o",
			Provider:       "openai",
			IsStream:       false,
			StatusCode:     200,
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
			IsStream:       false,
			StatusCode:     200,
		},
	}
	for i, r := range rows {
		if _, err := s.InsertRequestLog(ctx, r); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// No filter: all three, newest first.
	all, total, err := s.ListRequestLogs(ctx, RequestLogFilter{})
	if err != nil {
		t.Fatalf("ListRequestLogs all: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("expected 3 rows, got len=%d total=%d", len(all), total)
	}
	if all[0].VirtualKeyID != "vk-b" {
		t.Errorf("newest row should be vk-b, got %s", all[0].VirtualKeyID)
	}

	// Filter by key.
	keyA, _, err := s.ListRequestLogs(ctx, RequestLogFilter{VirtualKeyID: "vk-a"})
	if err != nil {
		t.Fatalf("filter by key: %v", err)
	}
	if len(keyA) != 2 {
		t.Errorf("expected 2 vk-a rows, got %d", len(keyA))
	}

	// Filter by status range: 5xx only.
	errs, _, err := s.ListRequestLogs(ctx, RequestLogFilter{StatusMin: 500, StatusMax: 599})
	if err != nil {
		t.Fatalf("filter by status: %v", err)
	}
	if len(errs) != 1 || errs[0].StatusCode != 500 {
		t.Errorf("expected 1 5xx row, got %+v", errs)
	}

	// Filter by stream flag (pointer to true).
	streamOnly := true
	streams, _, err := s.ListRequestLogs(ctx, RequestLogFilter{Stream: &streamOnly})
	if err != nil {
		t.Fatalf("filter by stream: %v", err)
	}
	if len(streams) != 1 || !streams[0].IsStream {
		t.Errorf("expected 1 stream row, got %+v", streams)
	}

	// Free-text search across the error column.
	matched, _, err := s.ListRequestLogs(ctx, RequestLogFilter{Search: "timeout"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matched) != 1 || matched[0].Error != "upstream timeout" {
		t.Errorf("expected 1 error-text match, got %+v", matched)
	}

	// Pagination: limit=1, offset=1 returns the middle row by time.
	page, _, err := s.ListRequestLogs(ctx, RequestLogFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(page) != 1 || page[0].VirtualKeyID != "vk-a" || !page[0].IsStream {
		t.Errorf("expected middle (vk-a stream) row, got %+v", page)
	}
}

func TestRequestLogGetMissing(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.GetRequestLog(ctx, "does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
