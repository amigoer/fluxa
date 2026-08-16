// Package gateway is the request-proxying runtime path described in
// DESIGN.md 4: authenticate -> DLP filter (request only) -> routing
// decision -> call the provider, streaming the response back -> record
// spend -> write the audit trail. This is the actual "gateway" in
// Fluxa's name; everything else in the codebase is management UI around
// it.
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	auditservice "github.com/amigoer/fluxa/internal/audit/service"
	audittypes "github.com/amigoer/fluxa/internal/audit/types"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/routing"
	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/security"
)

// longInputTokenThreshold is what pushes a request into the "长文本"
// routing condition from the mockup automatically, without the caller
// having to tag it.
const longInputTokenThreshold = 50_000

type Pipeline struct {
	providers *provider.Service
	security  *security.Service
	audit     auditservice.Service
	upstream  *upstreamClient
}

func NewPipeline(providers *provider.Service, sec *security.Service, aud auditservice.Service) *Pipeline {
	return &Pipeline{providers: providers, security: sec, audit: aud, upstream: newUpstreamClient()}
}

func (p *Pipeline) RegisterRoutes(r chi.Router) {
	r.Post("/v1/chat/completions", p.handleChatCompletions)
}

func (p *Pipeline) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// -- authenticate: virtual key, hashed lookup behind keyauth's cache
	secret := bearerToken(r)
	if secret == "" {
		httpx.Error(w, http.StatusUnauthorized, i18n.KeyVirtualKeyInvalid, "")
		return
	}
	key, err := p.providers.Keys.Authenticate(ctx, secret)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, i18n.KeyVirtualKeyInvalid, "")
		return
	}
	if key.Status != types.VirtualKeyStatusActive {
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyVirtualKeyRevoked, "")
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, i18n.KeyValidationFailed, err.Error())
		return
	}

	// -- DLP: request content only (see the security package doc comment
	// for why the response is never scanned)
	inputText := joinMessageContent(req.Messages)
	scan, err := p.security.Scan(ctx, inputText)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if scan.Blocked {
		for _, hit := range scan.Hits {
			_ = p.security.LogEvent(ctx, memberIDPtr(key), &key.ID, hit)
		}
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyValidationFailed, "request blocked by a DLP rule")
		return
	}
	for _, hit := range scan.Hits {
		_ = p.security.LogEvent(ctx, memberIDPtr(key), &key.ID, hit)
	}
	req.Messages = replaceMessageContent(req.Messages, scan.MaskedText)

	// -- routing: resolve target model, respecting health and the
	// personal/global fallback chain
	estimatedInputTokens := estimateTokens(scan.MaskedText)
	estimatedOutputTokens := req.MaxTokens
	if estimatedOutputTokens == 0 {
		estimatedOutputTokens = estimatedInputTokens // rough symmetric guess when the caller didn't say
	}

	condition := routingCondition(r, estimatedInputTokens)
	resolved, err := p.providers.Resolver.Resolve(ctx, memberIDOrEmpty(key), condition, estimatedInputTokens, estimatedOutputTokens)
	if err != nil {
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusServiceUnavailable, i18n.KeyProviderUnavailable, err.Error())
		return
	}

	if !modelInScope(key, resolved.Model.ID) {
		p.logCall(ctx, key, "", resolved.Model.ID, r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyModelNotEnabled, "")
		return
	}

	providerRec, err := p.providers.GetProvider(ctx, resolved.Model.ProviderID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	// -- call upstream, streaming the response straight through
	start := time.Now()
	result, callErr := p.upstream.forward(ctx, providerRec, resolved.Model, req, w)
	latency := time.Since(start)

	if callErr != nil || !result.StatusSuccess {
		_ = p.providers.Breaker.RecordFailure(ctx, providerRec.ID)
		p.logCall(ctx, key, providerRec.ID, resolved.Model.ID, r, latency, 0, 0, 0, audittypes.CallStatusFailed)
		return
	}

	_ = p.providers.Breaker.RecordSuccess(ctx, providerRec.ID)

	inputTokens, outputTokens := estimatedInputTokens, estimatedOutputTokens
	if result.Usage != nil {
		inputTokens, outputTokens = result.Usage.PromptTokens, result.Usage.CompletionTokens
	}
	costCents := routing.EstimateCostCents(resolved.Model, inputTokens, outputTokens)

	if ok, err := p.providers.SpendFromVirtualKey(ctx, key.ID, costCents); err == nil && !ok {
		// Budget was exceeded by the time this call completed (a race
		// with another concurrent call against the same key); the
		// response has already been streamed to the caller, so there is
		// nothing to roll back -- this is logged, not blocked
		// after-the-fact.
		_ = p.audit.LogOperation(ctx, "", "quota_exceeded_after_call", "key "+key.ID)
	}

	p.logCall(ctx, key, providerRec.ID, resolved.Model.ID, r, latency, inputTokens, outputTokens, costCents, audittypes.CallStatusSuccess)
}

func (p *Pipeline) logCall(ctx context.Context, key types.VirtualKey, providerID, modelID string, r *http.Request, latency time.Duration, inputTokens, outputTokens int, costCents int64, status audittypes.CallStatus) {
	memberID := ""
	if key.OwnerMemberID != nil {
		memberID = *key.OwnerMemberID
	}
	// Deliberately not discarded: swallowing this is what hid department-
	// owned keys failing to log at all (their member_id was an empty
	// string against a NOT NULL uuid column). The request itself has
	// already been answered, so a failure here is reported, not returned.
	if err := p.audit.LogCall(ctx, audittypes.CallLog{
		MemberID:     memberID,
		VirtualKeyID: key.ID,
		ProviderID:   providerID,
		ModelID:      modelID,
		RequestID:    r.Header.Get("X-Request-Id"),
		Status:       status,
		LatencyMS:    int(latency.Milliseconds()),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostCents:    costCents,
	}); err != nil {
		slog.ErrorContext(ctx, "gateway: write call log", "error", err, "virtual_key", key.ID)
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func joinMessageContent(messages []chatMessage) string {
	parts := make([]string, len(messages))
	for i, m := range messages {
		parts[i] = m.Content
	}
	return strings.Join(parts, "\n")
}

// replaceMessageContent re-splits the scanned, masked text back across
// the original messages. Because DLP replacement never changes line
// count (matched substrings are replaced in place, not the newlines
// joining messages), splitting on "\n" the same way joinMessageContent
// joined always yields the right number of parts back.
func replaceMessageContent(messages []chatMessage, maskedJoined string) []chatMessage {
	parts := strings.Split(maskedJoined, "\n")
	if len(parts) != len(messages) {
		return messages
	}
	out := make([]chatMessage, len(messages))
	for i, m := range messages {
		out[i] = chatMessage{Role: m.Role, Content: parts[i]}
	}
	return out
}

func memberIDOrEmpty(key types.VirtualKey) string {
	if key.OwnerMemberID != nil {
		return *key.OwnerMemberID
	}
	return ""
}

func memberIDPtr(key types.VirtualKey) *string {
	return key.OwnerMemberID
}

func modelInScope(key types.VirtualKey, modelID string) bool {
	if key.ModelScope == nil {
		return true // nil scope means every enabled model is allowed
	}
	for _, id := range key.ModelScope {
		if id == modelID {
			return true
		}
	}
	return false
}

// estimateTokens is a rough, fast approximation (roughly 4 characters
// per token for English; conservative enough for routing/cost-ceiling
// decisions, which only need to be in the right ballpark, not exact).
func estimateTokens(text string) int {
	return len([]rune(text))/4 + 1
}

func routingCondition(r *http.Request, estimatedInputTokens int) string {
	if estimatedInputTokens > longInputTokenThreshold {
		return "长文本（>50K）"
	}
	if hint := r.Header.Get("X-Fluxa-Route-Condition"); hint != "" {
		return hint
	}
	return "默认"
}
