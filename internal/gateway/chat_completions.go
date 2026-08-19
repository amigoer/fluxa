package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	audittypes "github.com/amigoer/fluxa/internal/audit/types"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/types"
)

// POST /v1/chat/completions -- the OpenAI-shaped entry point, and the
// one every other endpoint's translation ends up in.
func (p *Pipeline) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	key, ok := p.authenticate(w, r)
	if !ok {
		return
	}

	req, err := decodeChatRequest(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}

	if !p.scan(w, r, key, req.Messages) {
		return
	}

	adm, ok := p.admit(w, r, key, req.Messages, req.MaxTokens)
	if !ok {
		return
	}

	// Nothing below may return without closing the reservation out, or
	// the key keeps paying for a call that already ended.
	settled := false
	defer func() {
		if !settled {
			p.release(r.Context(), adm.Reservation)
		}
	}()

	start := time.Now()
	result, callErr := p.upstream.forward(r.Context(), adm.Provider, adm.Model, req, w)
	settled = p.settle(r, adm, result, callErr, time.Since(start))
}

func (p *Pipeline) logCall(ctx context.Context, key types.VirtualKey, providerID, modelID string, r *http.Request, latency time.Duration, inputTokens, outputTokens int, costMicroCents int64, status audittypes.CallStatus) {
	memberID := ""
	if key.OwnerMemberID != nil {
		memberID = *key.OwnerMemberID
	}
	// Deliberately not discarded: swallowing this is what hid department-
	// owned keys failing to log at all (their member_id was an empty
	// string against a NOT NULL uuid column). The request itself has
	// already been answered, so a failure here is reported, not returned.
	if err := p.audit.LogCall(ctx, audittypes.CallLog{
		MemberID:       memberID,
		VirtualKeyID:   key.ID,
		ProviderID:     providerID,
		ModelID:        modelID,
		RequestID:      r.Header.Get("X-Request-Id"),
		Status:         status,
		LatencyMS:      int(latency.Milliseconds()),
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		CostMicroCents: costMicroCents,
	}); err != nil {
		slog.ErrorContext(ctx, "gateway: write call log", "error", err, "virtual_key", key.ID)
	}
}
