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

	"github.com/amigoer/fluxa/internal/keys"
	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/router"
	"github.com/amigoer/fluxa/internal/store"
)

// Server bundles the dependencies needed by every HTTP handler. It is
// constructed once at startup and installed onto an http.ServeMux.
type Server struct {
	router  *router.Router
	logger  *slog.Logger
	keyring *keys.Keyring // optional: nil disables virtual key auth
	store   *store.Store  // optional: nil disables usage recording
}

// New returns a Server wired to the supplied router. A zero-value logger is
// replaced with the default slog logger so callers can pass nil in tests.
// Passing nil for keyring / store keeps the legacy "no virtual keys"
// behaviour, which is useful for the existing API tests.
func New(r *router.Router, logger *slog.Logger, kr *keys.Keyring, st *store.Store) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{router: r, logger: logger, keyring: kr, store: st}
}

// Routes installs the HTTP handlers onto mux. Keeping the mux mutation in
// one place makes it trivial to add middleware layers in a future commit
// without hunting for each handler registration.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/messages", s.handleMessages)
	mux.HandleFunc("GET /v1/models", s.handleListModels)
	mux.HandleFunc("GET /health", s.handleHealth)
}

// authorize runs virtual key policy checks on an inbound request. It
// returns the resolved key id (possibly empty when no virtual keys are
// active) and a typed error on denial.
//
// Three auth modes are supported:
//
//  1. No virtual keys configured — request passes through untouched; the
//     upstream API key is the provider key loaded from config.
//  2. Bearer token starts with "vk-" — token is looked up in the keyring
//     and every policy check runs.
//  3. Bearer token is anything else — treated as a direct pass-through
//     (operator chose to expose the gateway without key management).
//
// When any virtual keys exist in the ring, unauthenticated requests are
// rejected so operators do not accidentally leave the gateway wide open
// the moment they issue their first vk.
func (s *Server) authorize(r *http.Request, model string) (string, error) {
	if s.keyring == nil || s.keyring.Empty() {
		return "", nil
	}
	token, ok := keys.ExtractBearer(r.Header.Get("Authorization"))
	if !ok || !keys.IsVirtualKey(token) {
		return "", &provider.Error{Status: http.StatusUnauthorized, Message: "missing or invalid virtual key"}
	}
	if _, err := s.keyring.Authorize(r.Context(), token, model, r.RemoteAddr); err != nil {
		var denied *keys.Denied
		if errors.As(err, &denied) {
			return token, &provider.Error{Status: denied.Status, Message: denied.Reason}
		}
		return token, err
	}
	return token, nil
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
