// admin_config.go exposes YAML import/export endpoints for the whole
// provider + route state. Since v2.1 the gateway no longer boots from a
// YAML file, but operators still want a one-file snapshot format for
// backup, diffing and bulk edits. These endpoints close the loop:
//
//   GET  /admin/config/export  → serialise the current store as YAML
//   POST /admin/config/import  → parse a YAML bundle and upsert it
//
// The shape is the same Bundle that internal/config/yaml.go defines, so
// documents produced by export round-trip through import unchanged and
// legacy fluxa.yaml files from v1.x are still accepted.

package api

import (
	"io"
	"net/http"

	"github.com/amigoer/fluxa/internal/config"
)

// exportConfig writes the current providers + routes as a YAML bundle.
// The response uses application/yaml so browsers download it as a file
// rather than try to render it inline.
func (a *AdminServer) exportConfig(w http.ResponseWriter, r *http.Request) {
	provs, routes, err := a.store.LoadRouterInputs(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	raw, err := config.Marshal(provs, routes)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fluxa.yaml"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// importConfig parses a YAML bundle from the request body and upserts
// every provider and route into the store. It then triggers a router
// reload plus a keyring refresh so the live data plane immediately
// reflects the import. Existing rows not mentioned in the bundle are
// left alone, making the endpoint additive by design.
func (a *AdminServer) importConfig(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	bundle, err := config.Unmarshal(raw)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.Import(r.Context(), bundle.Providers, bundle.Routes); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "import: "+err.Error())
		return
	}
	if err := a.reloadRouter(r.Context()); err != nil {
		writeAdminError(w, http.StatusBadRequest, "reload: "+err.Error())
		return
	}
	if a.keyring != nil {
		if err := a.keyring.Reload(r.Context()); err != nil {
			a.logger.Warn("keyring reload after import failed", "err", err)
		}
	}
	a.logger.Info("admin config imported", "providers", len(bundle.Providers), "routes", len(bundle.Routes))
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "imported",
		"providers": len(bundle.Providers),
		"routes":    len(bundle.Routes),
	})
}
