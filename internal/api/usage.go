// usage.go — helpers that extract token counts from upstream responses
// and persist a store.UsageRecord row. The chat and messages handlers
// call recordUsage after a successful non-streaming response so operator
// budgets and dashboards stay in sync with real traffic.
//
// Streaming responses are not yet accounted for: SSE usage parsing is
// provider-specific and will land in a follow-up commit. The keyring's
// rate-limiter still runs on stream requests, so operators retain
// coarse-grained control.

package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/amigoer/fluxa/internal/pricing"
	"github.com/amigoer/fluxa/internal/store"
)

// usageExtractor pulls (prompt, completion, total) tokens out of a raw
// upstream response body. Different wire formats (OpenAI vs Anthropic)
// need different extractors, so handlers pass the right one explicitly.
type usageExtractor func(raw []byte) (prompt, completion, total int)

// recordUsage is a no-op when the server has no store configured or no
// virtual key is attached to the request. Otherwise it parses tokens,
// computes the USD cost from the built-in price table, inserts a row,
// and invalidates the keyring's cached totals so the next budget check
// sees fresh numbers.
func (s *Server) recordUsage(
	ctx context.Context,
	keyID, model, providerName string,
	raw []byte,
	started time.Time,
	status int,
	extract usageExtractor,
) {
	if s.store == nil || keyID == "" {
		return
	}
	prompt, completion, total := extract(raw)
	if total == 0 {
		total = prompt + completion
	}
	cost := pricing.Cost(model, prompt, completion)
	rec := store.UsageRecord{
		VirtualKeyID:     keyID,
		Ts:               time.Now(),
		Model:            model,
		Provider:         providerName,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		CostUSD:          cost,
		LatencyMs:        int(time.Since(started) / time.Millisecond),
		Status:           status,
	}
	if err := s.store.InsertUsage(ctx, rec); err != nil {
		s.logger.Warn("record usage", "err", err, "key", keyID, "model", model)
		return
	}
	if s.keyring != nil {
		s.keyring.InvalidateUsage(keyID)
	}
}

// usageFromOpenAI matches the shape of the OpenAI chat.completions
// response: {"usage":{"prompt_tokens":..,"completion_tokens":..,"total_tokens":..}}.
func usageFromOpenAI(raw []byte) (int, int, int) {
	var env struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, 0, 0
	}
	return env.Usage.PromptTokens, env.Usage.CompletionTokens, env.Usage.TotalTokens
}

// usageFromAnthropic matches the shape of the Anthropic messages
// response: {"usage":{"input_tokens":..,"output_tokens":..}}.
func usageFromAnthropic(raw []byte) (int, int, int) {
	var env struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, 0, 0
	}
	return env.Usage.InputTokens, env.Usage.OutputTokens, env.Usage.InputTokens + env.Usage.OutputTokens
}
