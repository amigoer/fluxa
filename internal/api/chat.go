package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/provider"
)

// handleChatCompletions implements POST /v1/chat/completions. It resolves
// the request to a provider chain, tries the primary first, and walks the
// fallback list on retryable errors. Streaming and non-streaming responses
// take clearly separated code paths because the lifecycle of an SSE writer
// is fundamentally different from a buffered JSON response.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20)) // 10 MiB
	if err != nil {
		s.writeError(w, &provider.Error{Status: http.StatusBadRequest, Message: "read request body: " + err.Error()})
		return
	}

	var peek struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		s.writeError(w, &provider.Error{Status: http.StatusBadRequest, Message: "invalid json body: " + err.Error()})
		return
	}
	if peek.Model == "" {
		s.writeError(w, &provider.Error{Status: http.StatusBadRequest, Message: "model is required"})
		return
	}

	// From this point on the request is well-formed enough to be worth
	// logging. The recorder is wired up before authorize/resolver/DLP
	// so every early-exit code path still produces a request_logs row.
	rec := s.newRequestRecorder(r, "/v1/chat/completions", body, peek.Model, peek.Stream, usageFromOpenAI)
	defer rec.flush()
	w = wrapRecording(w, rec)

	keyID, err := s.authorize(r, peek.Model)
	if err != nil {
		rec.markError(err)
		s.writeError(w, err)
		return
	}
	rec.setKey(keyID)

	// Pre-resolver: virtual_models / regex_models can rewrite the
	// incoming model name (and optionally pin a provider) before the
	// legacy provider chain lookup runs. A nil target means the resolver
	// declined to intervene and we should use peek.Model verbatim.
	target, _, err := s.router.ResolveModel(peek.Model)
	if err != nil {
		rec.markError(err)
		s.writeError(w, err)
		return
	}
	effectiveModel := peek.Model
	var chain []provider.Provider
	if target != nil {
		effectiveModel = target.Model
		chain, err = s.router.ResolveTargetChain(target)
	} else {
		chain, err = s.router.Resolve(peek.Model)
	}
	if err != nil {
		rec.markError(err)
		s.writeError(w, err)
		return
	}
	rec.setResolvedModel(effectiveModel)

	// Rewrite the request body so the upstream sees the resolved model
	// name. We only touch the "model" field — every other field stays
	// byte-for-byte identical so streaming/tool-call payloads are not
	// disturbed.
	if effectiveModel != peek.Model {
		body, err = rewriteModelField(body, effectiveModel)
		if err != nil {
			pErr := &provider.Error{Status: http.StatusInternalServerError, Message: "rewrite model: " + err.Error()}
			rec.markError(pErr)
			s.writeError(w, pErr)
			return
		}
	}

	// DLP request scan — inspect message content for sensitive data
	// patterns before it reaches any upstream provider.
	if s.dlpEngine != nil {
		result := s.dlpEngine.ScanRequest(r.Context(), body, effectiveModel, keyID)
		switch result.Action {
		case "block":
			go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, effectiveModel, "request")
			pErr := &provider.Error{Status: http.StatusForbidden, Message: "request blocked by DLP policy"}
			rec.markError(pErr)
			s.writeError(w, pErr)
			return
		case "mask":
			go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, effectiveModel, "request")
			body = result.Masked
		case "log":
			go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, effectiveModel, "request")
		}
	}

	req := &provider.ChatRequest{
		Model:  effectiveModel,
		Stream: peek.Stream,
		Raw:    body,
	}

	if peek.Stream {
		s.streamChat(w, r, chain, req, keyID, rec)
		return
	}
	s.nonStreamChat(w, r, chain, req, keyID, rec)
}

// nonStreamChat walks the fallback chain for a buffered chat completion.
func (s *Server) nonStreamChat(w http.ResponseWriter, r *http.Request, chain []provider.Provider, req *provider.ChatRequest, keyID string, rec *requestRecorder) {
	started := time.Now()
	var lastErr error
	for _, p := range chain {
		resp, err := p.Chat(r.Context(), req)
		if err == nil {
			// DLP response scan — inspect provider output before it
			// reaches the client.
			raw := resp.Raw
			if s.dlpEngine != nil {
				result := s.dlpEngine.ScanResponse(r.Context(), raw, req.Model, keyID)
				switch result.Action {
				case "block":
					go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, req.Model, "response")
					pErr := &provider.Error{Status: http.StatusForbidden, Message: "response blocked by DLP policy"}
					rec.setProvider(p.Name())
					rec.markError(pErr)
					s.writeError(w, pErr)
					return
				case "mask":
					go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, req.Model, "response")
					raw = result.Masked
				case "log":
					go s.dlpEngine.RecordViolations(context.Background(), result.Violations, keyID, req.Model, "response")
				}
			}
			rec.setProvider(p.Name())
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Fluxa-Provider", p.Name())
			_, _ = w.Write(raw)
			s.recordUsage(r.Context(), keyID, req.Model, p.Name(), resp.Raw, started, http.StatusOK, usageFromOpenAI)
			return
		}
		lastErr = err
		if !isRetryable(err) {
			s.logger.Warn("chat completion failed, no fallback", "provider", p.Name(), "err", err)
			rec.setProvider(p.Name())
			rec.markError(err)
			s.writeError(w, err)
			return
		}
		s.logger.Warn("chat completion failed, trying fallback", "provider", p.Name(), "err", err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no providers available")
	}
	rec.markError(lastErr)
	s.writeError(w, lastErr)
}

// streamChat walks the fallback chain for a streaming chat completion. Once
// the first byte is flushed to the client we can no longer fall back, so
// retries only happen before ChatStream returns successfully.
func (s *Server) streamChat(w http.ResponseWriter, r *http.Request, chain []provider.Provider, req *provider.ChatRequest, keyID string, rec *requestRecorder) {
	_ = keyID // streaming usage accounting is still TODO; see nonStreamChat for the buffered path. The request_log row captures the call regardless.
	flusher, ok := w.(http.Flusher)
	if !ok {
		pErr := &provider.Error{Status: http.StatusInternalServerError, Message: "streaming unsupported by server"}
		rec.markError(pErr)
		s.writeError(w, pErr)
		return
	}

	var (
		stream  <-chan provider.StreamEvent
		active  provider.Provider
		lastErr error
	)
	for _, p := range chain {
		ch, err := p.ChatStream(r.Context(), req)
		if err == nil {
			stream = ch
			active = p
			break
		}
		lastErr = err
		if !isRetryable(err) {
			rec.setProvider(p.Name())
			rec.markError(err)
			s.writeError(w, err)
			return
		}
		s.logger.Warn("chat stream setup failed, trying fallback", "provider", p.Name(), "err", err)
	}
	if stream == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("no providers available")
		}
		rec.markError(lastErr)
		s.writeError(w, lastErr)
		return
	}

	rec.setProvider(active.Name())
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.Header().Set("X-Fluxa-Provider", active.Name())
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for event := range stream {
		if event.Err != nil {
			s.logger.Warn("stream aborted by upstream", "provider", active.Name(), "err", event.Err)
			rec.markError(event.Err)
			// Emit an SSE error frame the client can parse; we cannot
			// change the HTTP status at this point.
			fmt.Fprintf(w, "data: {\"error\":{\"message\":%q}}\n\n", event.Err.Error())
			flusher.Flush()
			return
		}
		// Every chunk already contains the raw JSON payload; we wrap it in
		// the "data:" prefix and the SSE framing bytes.
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(event.Data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}
}

// isRetryable reports whether an error from a provider justifies trying the
// next fallback. Client errors (4xx) are not retried because they indicate
// a bad request rather than an upstream problem.
func isRetryable(err error) bool {
	var pErr *provider.Error
	if !errors.As(err, &pErr) {
		return true // network/unknown errors retry
	}
	if pErr.Status == 0 || pErr.Status >= 500 {
		return true
	}
	return pErr.Status == http.StatusTooManyRequests
}
