// Package httpx holds small, shared HTTP helpers used by every module's
// handler package: consistent JSON encoding and a single error shape carrying
// an i18n.Key instead of hardcoded prose.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/i18n"
)

// APIError is the JSON body returned for any non-2xx response. Key is
// what the frontend uses to render localized text; Detail is optional,
// non-localized context for logs and debugging, never shown to the user
// verbatim.
type APIError struct {
	Key    i18n.Key `json:"key"`
	Detail string   `json:"detail,omitempty"`
}

// JSON writes v as a JSON body with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("httpx: encode response failed", "error", err)
	}
}

// Error writes an APIError body with the given status code.
func Error(w http.ResponseWriter, status int, key i18n.Key, detail string) {
	JSON(w, status, APIError{Key: key, Detail: detail})
}

// InternalError logs the underlying error and writes a generic 500 body
// that does not leak internals to the client.
func InternalError(w http.ResponseWriter, err error) {
	slog.Default().Error("httpx: internal error", "error", err)
	Error(w, http.StatusInternalServerError, i18n.KeyInternalError, "")
}
