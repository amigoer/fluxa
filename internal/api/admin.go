// admin.go exposes REST endpoints for provider and route CRUD. Every
// mutation writes through the store, then triggers router.Reload so the
// gateway picks up the change with zero downtime.
//
// Authentication: every /admin/* request other than /admin/auth/login
// must present
//
//	Authorization: Bearer <session_token>
//
// where session_token was minted by /admin/auth/login. The token is
// resolved against the admin_sessions table on every call so a logout
// invalidates instantly. There is no longer a static master key in env
// vars — operators sign in with a username + password instead.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/amigoer/fluxa/internal/keys"
	"github.com/amigoer/fluxa/internal/router"
	"github.com/amigoer/fluxa/internal/store"
)

// adminCtxKey is the private context key under which requireAuth stows
// the resolved AdminUser. Handlers downstream pull it back out via
// userFromContext.
type adminCtxKey struct{}

// userFromContext returns the authenticated user attached by requireAuth.
// Handlers that need the caller (e.g. me, changePassword) read it here
// rather than re-querying the database on every call.
func userFromContext(r *http.Request) (store.AdminUser, bool) {
	v, ok := r.Context().Value(adminCtxKey{}).(store.AdminUser)
	return v, ok
}

// AdminServer owns the admin REST endpoints. It is constructed once at
// startup and installed alongside the data-plane routes on the same mux.
type AdminServer struct {
	router  *router.Router
	store   *store.Store
	keyring *keys.Keyring // optional: nil disables virtual-key admin reloads
	logger  *slog.Logger
}

// NewAdmin returns a ready-to-wire AdminServer. Authentication is now
// session-token based; the admin_users table is the source of truth for
// who can sign in, so this constructor takes no master key. Keyring is
// optional — when nil the virtual-key endpoints still work, but no
// in-memory cache is invalidated so the data plane will lag the store
// by at most one process lifetime.
func NewAdmin(r *router.Router, s *store.Store, kr *keys.Keyring, logger *slog.Logger) *AdminServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdminServer{router: r, store: s, keyring: kr, logger: logger}
}

// Routes installs the admin handlers onto mux. They share the mux with the
// data-plane API so one process listens on a single port.
func (a *AdminServer) Routes(mux *http.ServeMux) {
	// Auth surface — login is the only unauthenticated endpoint.
	mux.HandleFunc("POST /admin/auth/login", a.login)
	mux.HandleFunc("POST /admin/auth/logout", a.requireAuth(a.logout))
	mux.HandleFunc("GET /admin/auth/me", a.requireAuth(a.me))
	mux.HandleFunc("POST /admin/auth/password", a.requireAuth(a.changePassword))

	mux.HandleFunc("GET /admin/providers", a.requireAuth(a.listProviders))
	mux.HandleFunc("POST /admin/providers", a.requireAuth(a.upsertProvider))
	mux.HandleFunc("GET /admin/providers/{name}", a.requireAuth(a.getProvider))
	mux.HandleFunc("PUT /admin/providers/{name}", a.requireAuth(a.upsertProvider))
	mux.HandleFunc("DELETE /admin/providers/{name}", a.requireAuth(a.deleteProvider))

	mux.HandleFunc("GET /admin/routes", a.requireAuth(a.listRoutes))
	mux.HandleFunc("POST /admin/routes", a.requireAuth(a.upsertRoute))
	mux.HandleFunc("GET /admin/routes/{model...}", a.requireAuth(a.getRoute))
	mux.HandleFunc("PUT /admin/routes/{model...}", a.requireAuth(a.upsertRoute))
	mux.HandleFunc("DELETE /admin/routes/{model...}", a.requireAuth(a.deleteRoute))

	mux.HandleFunc("GET /admin/keys", a.requireAuth(a.listKeys))
	mux.HandleFunc("POST /admin/keys", a.requireAuth(a.createKey))
	mux.HandleFunc("GET /admin/keys/{id}", a.requireAuth(a.getKey))
	mux.HandleFunc("PUT /admin/keys/{id}", a.requireAuth(a.updateKey))
	mux.HandleFunc("DELETE /admin/keys/{id}", a.requireAuth(a.deleteKey))

	mux.HandleFunc("GET /admin/usage", a.requireAuth(a.listUsage))
	mux.HandleFunc("GET /admin/usage/summary", a.requireAuth(a.usageSummary))

	mux.HandleFunc("GET /admin/config/export", a.requireAuth(a.exportConfig))
	mux.HandleFunc("POST /admin/config/import", a.requireAuth(a.importConfig))

	mux.HandleFunc("POST /admin/reload", a.requireAuth(a.reload))
}

// requireAuth is the Bearer-token middleware used by every admin endpoint
// other than /admin/auth/login. It resolves the bearer token to a row in
// admin_sessions and stashes the joined AdminUser on the request
// context so downstream handlers can read it via userFromContext.
func (a *AdminServer) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeAdminError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		user, err := a.store.LookupSession(r.Context(), token)
		if err != nil {
			writeAdminError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		ctx := context.WithValue(r.Context(), adminCtxKey{}, user)
		next(w, r.WithContext(ctx))
	}
}

// -- wire types ---------------------------------------------------------

// providerDTO is the JSON shape accepted and returned by the admin API. It
// is isomorphic to store.Provider minus timestamps on input.
type providerDTO struct {
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	APIKey       string            `json:"api_key,omitempty"`
	BaseURL      string            `json:"base_url,omitempty"`
	APIVersion   string            `json:"api_version,omitempty"`
	Region       string            `json:"region,omitempty"`
	AccessKey    string            `json:"access_key,omitempty"`
	SecretKey    string            `json:"secret_key,omitempty"`
	SessionToken string            `json:"session_token,omitempty"`
	Deployments  map[string]string `json:"deployments,omitempty"`
	Models       []string          `json:"models,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	TimeoutSec   int               `json:"timeout_sec,omitempty"`
	Enabled      *bool             `json:"enabled,omitempty"`
	CreatedAt    time.Time         `json:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

type routeDTO struct {
	Model     string    `json:"model"`
	Provider  string    `json:"provider"`
	Fallback  []string  `json:"fallback,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func toProviderDTO(p store.Provider) providerDTO {
	enabled := p.Enabled
	return providerDTO{
		Name:         p.Name,
		Kind:         p.Kind,
		APIKey:       p.APIKey,
		BaseURL:      p.BaseURL,
		APIVersion:   p.APIVersion,
		Region:       p.Region,
		AccessKey:    p.AccessKey,
		SecretKey:    p.SecretKey,
		SessionToken: p.SessionToken,
		Deployments:  p.Deployments,
		Models:       p.Models,
		Headers:      p.Headers,
		TimeoutSec:   p.TimeoutSec,
		Enabled:      &enabled,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func fromProviderDTO(dto providerDTO) store.Provider {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	return store.Provider{
		Name:         dto.Name,
		Kind:         dto.Kind,
		APIKey:       dto.APIKey,
		BaseURL:      dto.BaseURL,
		APIVersion:   dto.APIVersion,
		Region:       dto.Region,
		AccessKey:    dto.AccessKey,
		SecretKey:    dto.SecretKey,
		SessionToken: dto.SessionToken,
		Deployments:  dto.Deployments,
		Models:       dto.Models,
		Headers:      dto.Headers,
		TimeoutSec:   dto.TimeoutSec,
		Enabled:      enabled,
	}
}

func toRouteDTO(r store.Route) routeDTO {
	return routeDTO{
		Model:     r.Model,
		Provider:  r.Provider,
		Fallback:  r.Fallback,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func fromRouteDTO(dto routeDTO) store.Route {
	return store.Route{
		Model:    dto.Model,
		Provider: dto.Provider,
		Fallback: dto.Fallback,
	}
}

// -- provider handlers --------------------------------------------------

func (a *AdminServer) listProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListProviders(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]providerDTO, 0, len(rows))
	for _, p := range rows {
		out = append(out, toProviderDTO(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getProvider(w http.ResponseWriter, r *http.Request) {
	p, err := a.store.GetProvider(r.Context(), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "provider not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProviderDTO(p))
}

func (a *AdminServer) upsertProvider(w http.ResponseWriter, r *http.Request) {
	var dto providerDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	// PUT /admin/providers/{name} — use the URL name if the body omits it
	// so clients can send a minimal patch. POST always requires the body.
	if name := r.PathValue("name"); name != "" {
		dto.Name = name
	}
	if dto.Name == "" || dto.Kind == "" {
		writeAdminError(w, http.StatusBadRequest, "name and kind are required")
		return
	}
	row := fromProviderDTO(dto)
	if err := a.store.UpsertProvider(r.Context(), row); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.reloadRouter(r.Context()); err != nil {
		writeAdminError(w, http.StatusBadRequest, "reload failed: "+err.Error())
		return
	}
	a.logger.Info("admin provider upserted", "name", row.Name)
	out, _ := a.store.GetProvider(r.Context(), row.Name)
	writeJSON(w, http.StatusOK, toProviderDTO(out))
}

func (a *AdminServer) deleteProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := a.store.DeleteProvider(r.Context(), name); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "provider not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.reloadRouter(r.Context()); err != nil {
		// Deleting a provider that is still referenced by a route leaves
		// the store consistent but breaks reload. Surface the error so
		// the operator knows to clean up the dangling route.
		a.logger.Warn("router reload after delete failed", "provider", name, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- route handlers -----------------------------------------------------

func (a *AdminServer) listRoutes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListRoutes(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]routeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRouteDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getRoute(w http.ResponseWriter, r *http.Request) {
	row, err := a.store.GetRoute(r.Context(), r.PathValue("model"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "route not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRouteDTO(row))
}

func (a *AdminServer) upsertRoute(w http.ResponseWriter, r *http.Request) {
	var dto routeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if model := r.PathValue("model"); model != "" {
		dto.Model = model
	}
	if dto.Model == "" || dto.Provider == "" {
		writeAdminError(w, http.StatusBadRequest, "model and provider are required")
		return
	}
	row := fromRouteDTO(dto)
	if err := a.store.UpsertRoute(r.Context(), row); err != nil {
		// A FK constraint violation means the caller referenced a
		// provider that does not exist. Surface it as 400 so clients
		// distinguish operator mistakes from server faults.
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "constraint") {
			status = http.StatusBadRequest
		}
		writeAdminError(w, status, err.Error())
		return
	}
	if err := a.reloadRouter(r.Context()); err != nil {
		writeAdminError(w, http.StatusBadRequest, "reload failed: "+err.Error())
		return
	}
	a.logger.Info("admin route upserted", "model", row.Model, "provider", row.Provider)
	out, _ := a.store.GetRoute(r.Context(), row.Model)
	writeJSON(w, http.StatusOK, toRouteDTO(out))
}

func (a *AdminServer) deleteRoute(w http.ResponseWriter, r *http.Request) {
	model := r.PathValue("model")
	if err := a.store.DeleteRoute(r.Context(), model); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "route not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.reloadRouter(r.Context()); err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reload is a manual trigger that re-reads the store and rebuilds the
// router. Useful when an operator has mutated the database out-of-band.
func (a *AdminServer) reload(w http.ResponseWriter, r *http.Request) {
	if err := a.reloadRouter(r.Context()); err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reloaded"})
}

// reloadRouter is the single code path that rebuilds the router from the
// current store state. Every admin mutation calls it, plus the explicit
// /admin/reload endpoint.
func (a *AdminServer) reloadRouter(ctx context.Context) error {
	provs, routes, err := a.store.LoadRouterInputs(ctx)
	if err != nil {
		return err
	}
	return a.router.Reload(provs, routes)
}

// -- helpers ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAdminError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"message": msg, "type": http.StatusText(status)}})
}
