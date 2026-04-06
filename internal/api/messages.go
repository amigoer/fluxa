package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/amigoer/fluxa/internal/provider"
)

// handleMessages implements POST /v1/messages, the Anthropic-native entry
// point used by Claude Code and similar tools. Only providers that satisfy
// provider.MessagesProvider can serve this endpoint because we refuse to
// translate between wire formats for v1.0 — the whole point of /v1/messages
// is a lossless passthrough.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
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

	chain, err := s.router.Resolve(peek.Model)
	if err != nil {
		s.writeError(w, err)
		return
	}

	req := &provider.MessagesRequest{Model: peek.Model, Raw: body}
	if peek.Stream {
		s.streamMessages(w, r, chain, req)
		return
	}
	s.nonStreamMessages(w, r, chain, req)
}

// firstMessagesProvider walks the resolved chain and returns the first
// provider that implements the Anthropic-native interface along with the
// remaining fallback list. This keeps /v1/messages strict about protocol
// support without breaking /v1/chat/completions on the same provider list.
func firstMessagesProvider(chain []provider.Provider) (provider.MessagesProvider, []provider.MessagesProvider) {
	var primary provider.MessagesProvider
	var fallbacks []provider.MessagesProvider
	for _, p := range chain {
		mp, ok := p.(provider.MessagesProvider)
		if !ok {
			continue
		}
		if primary == nil {
			primary = mp
		} else {
			fallbacks = append(fallbacks, mp)
		}
	}
	return primary, fallbacks
}

func (s *Server) nonStreamMessages(w http.ResponseWriter, r *http.Request, chain []provider.Provider, req *provider.MessagesRequest) {
	primary, fallbacks := firstMessagesProvider(chain)
	if primary == nil {
		s.writeError(w, &provider.Error{Status: http.StatusNotImplemented, Message: "no Anthropic-compatible provider configured for this model"})
		return
	}

	attempts := append([]provider.MessagesProvider{primary}, fallbacks...)
	var lastErr error
	for _, p := range attempts {
		resp, err := p.Messages(r.Context(), req)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Fluxa-Provider", p.Name())
			_, _ = w.Write(resp.Raw)
			return
		}
		lastErr = err
		if !isRetryable(err) {
			s.writeError(w, err)
			return
		}
		s.logger.Warn("messages call failed, trying fallback", "provider", p.Name(), "err", err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no providers available")
	}
	s.writeError(w, lastErr)
}

func (s *Server) streamMessages(w http.ResponseWriter, r *http.Request, chain []provider.Provider, req *provider.MessagesRequest) {
	primary, fallbacks := firstMessagesProvider(chain)
	if primary == nil {
		s.writeError(w, &provider.Error{Status: http.StatusNotImplemented, Message: "no Anthropic-compatible provider configured for this model"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, &provider.Error{Status: http.StatusInternalServerError, Message: "streaming unsupported by server"})
		return
	}

	attempts := append([]provider.MessagesProvider{primary}, fallbacks...)
	var (
		stream  <-chan provider.StreamEvent
		active  provider.MessagesProvider
		lastErr error
	)
	for _, p := range attempts {
		ch, err := p.MessagesStream(r.Context(), req)
		if err == nil {
			stream = ch
			active = p
			break
		}
		lastErr = err
		if !isRetryable(err) {
			s.writeError(w, err)
			return
		}
		s.logger.Warn("messages stream setup failed, trying fallback", "provider", p.Name(), "err", err)
	}
	if stream == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("no providers available")
		}
		s.writeError(w, lastErr)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Fluxa-Provider", active.Name())
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for event := range stream {
		if event.Err != nil {
			s.logger.Warn("messages stream aborted", "provider", active.Name(), "err", event.Err)
			fmt.Fprintf(w, "event: error\ndata: {\"error\":{\"message\":%q}}\n\n", event.Err.Error())
			flusher.Flush()
			return
		}
		// Anthropic stream events are delivered as "eventName|payload" by
		// the adapter; split once and emit the canonical SSE frame that
		// Anthropic SDKs expect.
		eventName, payload := splitEventPayload(event.Data)
		if eventName != "" {
			_, _ = fmt.Fprintf(w, "event: %s\n", eventName)
		}
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(payload)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}
}

func splitEventPayload(buf []byte) (string, []byte) {
	idx := bytes.IndexByte(buf, '|')
	if idx < 0 {
		return "", buf
	}
	return string(buf[:idx]), buf[idx+1:]
}
