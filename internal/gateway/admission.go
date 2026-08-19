package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	audittypes "github.com/amigoer/fluxa/internal/audit/types"
	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/platform/i18n"
	"github.com/amigoer/fluxa/internal/provider/routing"
	"github.com/amigoer/fluxa/internal/provider/types"
	securityservice "github.com/amigoer/fluxa/internal/security/service"
)

// Everything the gateway decides before a request is allowed to reach a
// provider, and everything it has to do afterwards -- shared by every
// endpoint rather than written once per endpoint.
//
// It lives apart from the endpoints because the order is the product:
// authenticate, scan, route, check the ceiling, reserve, and only then
// call. An endpoint added later that reimplemented this would be one
// forgotten step away from being the hole the other endpoints do not
// have.

// admitted is a request that has passed every gate, holding what the
// settle step needs afterwards.
type admitted struct {
	Key      types.VirtualKey
	Model    types.Model
	Provider types.Provider

	// Reservation is the quota hold taken out for this call. It must be
	// closed exactly once, by Settle or Release.
	Reservation string

	InputTokens       int
	MaxOutputTokens   int
	MaxCostMicroCents int64
}

// authenticate resolves the bearer token to an active virtual key.
func (p *Pipeline) authenticate(w http.ResponseWriter, r *http.Request) (types.VirtualKey, bool) {
	secret := bearerToken(r)
	if secret == "" {
		httpx.Error(w, http.StatusUnauthorized, i18n.KeyVirtualKeyInvalid, "")
		return types.VirtualKey{}, false
	}
	key, err := p.providers.Keys().Authenticate(r.Context(), secret)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, i18n.KeyVirtualKeyInvalid, "")
		return types.VirtualKey{}, false
	}
	if key.Status != types.VirtualKeyStatusActive {
		p.logCall(r.Context(), key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyVirtualKeyRevoked, "")
		return types.VirtualKey{}, false
	}
	return key, true
}

// scan runs DLP over a request's messages, masking in place.
//
// One message at a time rather than over all of them joined into a
// single string: joining on "\n" and splitting the masked result back
// apart only lines up when no message contains a newline of its own, and
// any multi-line prompt -- a system prompt, pasted code, a pasted
// document -- made the split come back with the wrong number of parts,
// at which point the original *unmasked* messages went upstream while
// the security event log still recorded the hit. A masking failure that
// leaves the audit trail claiming success is worse than no masking at
// all, so there is no longer a path that can produce one.
func (p *Pipeline) scan(w http.ResponseWriter, r *http.Request, key types.VirtualKey, messages []chatMessage) bool {
	ctx := r.Context()

	blocked := false
	var hits []securityservice.Hit
	for i := range messages {
		result, err := p.security.Scan(ctx, messages[i].Content)
		if err != nil {
			httpx.InternalError(w, err)
			return false
		}
		hits = append(hits, result.Hits...)
		if result.Blocked {
			blocked = true
			break
		}
		messages[i].Content = result.MaskedText
	}
	for _, hit := range hits {
		_ = p.security.LogEvent(ctx, memberIDPtr(key), &key.ID, hit)
	}
	if blocked {
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyValidationFailed, "request blocked by a DLP rule")
		return false
	}
	return true
}

// admit routes a scanned request, checks it against every ceiling, and
// takes out the quota reservation that pays for it. On refusal it has
// already written the response and taken out nothing.
func (p *Pipeline) admit(w http.ResponseWriter, r *http.Request, key types.VirtualKey, messages []chatMessage, requestedMaxTokens int) (admitted, bool) {
	ctx := r.Context()

	inputTokens := estimateMessageTokens(messages)
	condition := routingCondition(r, inputTokens)

	resolved, err := p.providers.Resolver().Resolve(ctx, memberIDOrEmpty(key), condition, inputTokens, requestedMaxTokens)
	if errors.Is(err, routing.ErrCostCeilingExceeded) {
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyCostCeilingExceeded, "")
		return admitted{}, false
	}
	if err != nil {
		p.logCall(ctx, key, "", "", r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusServiceUnavailable, i18n.KeyProviderUnavailable, err.Error())
		return admitted{}, false
	}

	// What this call costs if the model emits everything it is allowed
	// to. Every gate below is decided against this rather than against a
	// likely figure: a budget check that assumes the typical case is not
	// a budget check.
	maxOutputTokens := routing.MaxOutputTokens(resolved.Model, requestedMaxTokens, inputTokens)

	adm, ok := p.admitModel(w, r, key, resolved.Model, inputTokens, maxOutputTokens)
	if !ok && resolved.ProbeClaimed {
		// Routing let this request through as the recovering provider's
		// single probe, and then a gate further down refused it -- out
		// of budget, model out of scope. The provider will never hear
		// how that probe went, so hand the slot back rather than hold it
		// shut until the probe times out. A key that is simply out of
		// budget would otherwise do this to every provider it routes to.
		if err := p.providers.Breaker().ReleaseProbe(context.WithoutCancel(ctx), resolved.Model.ProviderID); err != nil {
			slog.ErrorContext(ctx, "gateway: release provider probe", "error", err, "provider", resolved.Model.ProviderID)
		}
	}
	return adm, ok
}

// admitModel is admission from the point the model is already decided:
// the per-call ceiling, the key's model scope, and the quota
// reservation. /v1/embeddings comes in here directly, because a caller
// asking for embeddings names the model it wants rather than leaving the
// choice to a routing rule -- but it must still pass every gate the
// chat path does, so those live here rather than in the chat endpoint.
func (p *Pipeline) admitModel(w http.ResponseWriter, r *http.Request, key types.VirtualKey, model types.Model, inputTokens, maxOutputTokens int) (admitted, bool) {
	ctx := r.Context()

	maxCost := routing.EstimateCostMicroCents(model, inputTokens, maxOutputTokens)

	if p.maxRequestCostMicroCents > 0 && maxCost > p.maxRequestCostMicroCents {
		p.logCall(ctx, key, "", model.ID, r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyCostCeilingExceeded, "")
		return admitted{}, false
	}

	if !modelInScope(key, model.ID) {
		p.logCall(ctx, key, "", model.ID, r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusForbidden, i18n.KeyModelNotEnabled, "")
		return admitted{}, false
	}

	providerRec, err := p.providers.GetProvider(ctx, model.ProviderID)
	if err != nil {
		httpx.InternalError(w, err)
		return admitted{}, false
	}

	// Admission against the key's remaining budget, before any of the
	// provider's money is spent. Settling below the reserved figure
	// afterwards is free; discovering afterwards that there was never
	// room is not.
	reservation, ok, err := p.providers.ReserveQuota(ctx, key.ID, maxCost)
	if err != nil {
		httpx.InternalError(w, err)
		return admitted{}, false
	}
	if !ok {
		p.logCall(ctx, key, "", model.ID, r, 0, 0, 0, 0, audittypes.CallStatusFailed)
		httpx.Error(w, http.StatusPaymentRequired, i18n.KeyQuotaExceeded, "")
		return admitted{}, false
	}

	return admitted{
		Key:               key,
		Model:             model,
		Provider:          providerRec,
		Reservation:       reservation,
		InputTokens:       inputTokens,
		MaxOutputTokens:   maxOutputTokens,
		MaxCostMicroCents: maxCost,
	}, true
}

// release gives a reservation back without charging for it. Used by the
// deferred cleanup every endpoint installs, so no exit path -- an error,
// a panic unwinding through Recoverer -- can leave a key paying for a
// call that already ended.
//
// context.WithoutCancel: a client that hung up is exactly when this
// matters most, and running on the request's cancelled context would
// silently skip it.
func (p *Pipeline) release(ctx context.Context, reservation string) {
	if err := p.providers.ReleaseQuota(context.WithoutCancel(ctx), reservation); err != nil {
		slog.ErrorContext(ctx, "gateway: release quota reservation", "error", err, "reservation", reservation)
	}
}

// settle closes out an admitted call: records the breaker outcome,
// charges the reservation at what the call actually cost, and writes the
// audit line. It reports whether the reservation was closed here, which
// the caller's deferred release checks.
func (p *Pipeline) settle(r *http.Request, adm admitted, result outcome, callErr error, latency time.Duration) bool {
	ctx := r.Context()

	if callErr != nil || !result.StatusSuccess {
		if err := p.providers.Breaker().RecordFailure(ctx, adm.Provider.ID); err != nil {
			slog.ErrorContext(ctx, "gateway: record provider failure", "error", err, "provider", adm.Provider.ID)
		}
		p.logCall(ctx, adm.Key, adm.Provider.ID, adm.Model.ID, r, latency, 0, 0, 0, audittypes.CallStatusFailed)
		return false
	}

	if err := p.providers.Breaker().RecordSuccess(ctx, adm.Provider.ID); err != nil {
		slog.ErrorContext(ctx, "gateway: record provider success", "error", err, "provider", adm.Provider.ID)
	}

	// Both streaming and non-streaming report real usage. The fallback
	// is for a provider that reported none -- one predating
	// stream_options, or one this deployment opted out of asking -- and
	// it settles at what the call was admitted on rather than at a
	// likelier guess. That errs high on purpose: a lower guess would
	// leave the one traffic shape the gateway cannot measure as the one
	// it bills least.
	inputTokens, outputTokens := adm.InputTokens, adm.MaxOutputTokens
	costMicroCents := adm.MaxCostMicroCents
	if result.Usage != nil {
		inputTokens, outputTokens = result.Usage.PromptTokens, result.Usage.CompletionTokens
		costMicroCents = routing.EstimateCostMicroCents(adm.Model, inputTokens, outputTokens)
	}

	// The response has already gone to the caller, so this cannot refuse
	// anything -- but it no longer has to: admission happened before the
	// call, and charging the real figure here is what makes the next one
	// refuse.
	if err := p.providers.SettleQuota(context.WithoutCancel(ctx), adm.Reservation, costMicroCents); err != nil {
		slog.ErrorContext(ctx, "gateway: settle quota reservation", "error", err, "reservation", adm.Reservation)
	}

	p.logCall(ctx, adm.Key, adm.Provider.ID, adm.Model.ID, r, latency, inputTokens, outputTokens, costMicroCents, audittypes.CallStatusSuccess)
	return true
}
