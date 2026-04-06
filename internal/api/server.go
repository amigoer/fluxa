// Package api exposes the HTTP surface of the Fluxa gateway. It wires the
// router layer to OpenAI- and Anthropic-compatible endpoints and owns all of
// the request/response plumbing: JSON parsing, error envelopes, and SSE
// streaming.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/router"
)

// Server bundles the dependencies needed by every HTTP handler. It is
// constructed once at startup and installed onto an http.ServeMux.
type Server struct {
	router *router.Router
	logger *slog.Logger
}

// New returns a Server wired to the supplied router. A zero-value logger is
// replaced with the default slog logger so callers can pass nil in tests.
func New(r *router.Router, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{router: r, logger: logger}
}

// Routes installs the HTTP handlers onto mux. Keeping the mux mutation in
// one place makes it trivial to add middleware layers in a future commit
// without hunting for each handler registration.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
}

// errorEnvelope matches the OpenAI error shape so existing SDKs can surface
// Fluxa-originated errors without special casing.
type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// writeError serialises an error as an OpenAI-shaped JSON envelope. The
// status code is pulled from provider.Error when available so upstream 4xx
// pass through unchanged.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := ""
	msg := err.Error()
	var pErr *provider.Error
	if errors.As(err, &pErr) {
		status = pErr.Status
		code = pErr.Code
		msg = pErr.Message
	}
	if errors.Is(err, router.ErrUnknownModel) {
		status = http.StatusNotFound
	}
	var env errorEnvelope
	env.Error.Message = msg
	env.Error.Type = http.StatusText(status)
	env.Error.Code = code

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}
