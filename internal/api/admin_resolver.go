// admin_resolver.go exposes the control-plane endpoints that drive
// the dashboard's Virtual Models, Regex Models, and Resolve Tester
// pages. The handlers themselves are thin: they decode JSON, call into
// the store, then trigger the matching router.Reload* helper so the
// data plane picks up the change with zero downtime. The interesting
// logic lives in internal/store/{virtual_models,regex_models}.go and
// internal/router/model_resolver.go.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// -- wire types ---------------------------------------------------------

type virtualModelDTO struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty"`
	Routes      []virtualModelRouteDTO `json:"routes"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at,omitempty"`
}

type virtualModelRouteDTO struct {
	ID          string `json:"id,omitempty"`
	Weight      int    `json:"weight"`
	TargetType  string `json:"target_type"`
	TargetModel string `json:"target_model"`
	Provider    string `json:"provider,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Position    int    `json:"position,omitempty"`
}

type regexModelDTO struct {
	ID          string    `json:"id,omitempty"`
	Pattern     string    `json:"pattern"`
	Priority    int       `json:"priority"`
	TargetType  string    `json:"target_type"`
	TargetModel string    `json:"target_model"`
	Provider    string    `json:"provider,omitempty"`
	Description string    `json:"description,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

func toVirtualModelDTO(vm store.VirtualModel) virtualModelDTO {
	enabled := vm.Enabled
	out := virtualModelDTO{
		ID:          vm.ID,
		Name:        vm.Name,
		Description: vm.Description,
		Enabled:     &enabled,
		CreatedAt:   vm.CreatedAt,
		UpdatedAt:   vm.UpdatedAt,
		Routes:      make([]virtualModelRouteDTO, 0, len(vm.Routes)),
	}
	for _, rt := range vm.Routes {
		rEnabled := rt.Enabled
		out.Routes = append(out.Routes, virtualModelRouteDTO{
			ID:          rt.ID,
			Weight:      rt.Weight,
			TargetType:  rt.TargetType,
			TargetModel: rt.TargetModel,
			Provider:    rt.Provider,
			Enabled:     &rEnabled,
			Position:    rt.Position,
		})
	}
	return out
}

func fromVirtualModelDTO(dto virtualModelDTO) store.VirtualModel {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	vm := store.VirtualModel{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Enabled:     enabled,
	}
	for _, rt := range dto.Routes {
		rEnabled := true
		if rt.Enabled != nil {
			rEnabled = *rt.Enabled
		}
		vm.Routes = append(vm.Routes, store.VirtualModelRoute{
			Weight:      rt.Weight,
			TargetType:  rt.TargetType,
			TargetModel: rt.TargetModel,
			Provider:    rt.Provider,
			Enabled:     rEnabled,
		})
	}
	return vm
}

func toRegexModelDTO(r store.RegexModel) regexModelDTO {
	enabled := r.Enabled
	return regexModelDTO{
		ID:          r.ID,
		Pattern:     r.Pattern,
		Priority:    r.Priority,
		TargetType:  r.TargetType,
		TargetModel: r.TargetModel,
		Provider:    r.Provider,
		Description: r.Description,
		Enabled:     &enabled,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func fromRegexModelDTO(dto regexModelDTO) store.RegexModel {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	return store.RegexModel{
		ID:          dto.ID,
		Pattern:     dto.Pattern,
		Priority:    dto.Priority,
		TargetType:  dto.TargetType,
		TargetModel: dto.TargetModel,
		Provider:    dto.Provider,
		Description: dto.Description,
		Enabled:     enabled,
	}
}

// -- virtual model handlers --------------------------------------------

func (a *AdminServer) listVirtualModels(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListVirtualModels(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]virtualModelDTO, 0, len(rows))
	for _, vm := range rows {
		out = append(out, toVirtualModelDTO(vm))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getVirtualModel(w http.ResponseWriter, r *http.Request) {
	vm, err := a.store.GetVirtualModel(r.Context(), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "virtual model not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toVirtualModelDTO(vm))
}

func (a *AdminServer) upsertVirtualModel(w http.ResponseWriter, r *http.Request) {
	var dto virtualModelDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if name := r.PathValue("name"); name != "" {
		dto.Name = name
	}
	if dto.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "name is required")
		return
	}
	saved, err := a.store.UpsertVirtualModel(r.Context(), fromVirtualModelDTO(dto))
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.router.ReloadVirtualModels(r.Context()); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}
	a.logger.Info("admin virtual model upserted", "name", saved.Name)
	writeJSON(w, http.StatusOK, toVirtualModelDTO(saved))
}

func (a *AdminServer) deleteVirtualModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := a.store.DeleteVirtualModel(r.Context(), name); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "virtual model not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.router.ReloadVirtualModels(r.Context()); err != nil {
		a.logger.Warn("router reload after virtual model delete failed", "name", name, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- regex model handlers ----------------------------------------------

func (a *AdminServer) listRegexModels(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListRegexModels(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]regexModelDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRegexModelDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *AdminServer) getRegexModel(w http.ResponseWriter, r *http.Request) {
	row, err := a.store.GetRegexModel(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "regex model not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRegexModelDTO(row))
}

func (a *AdminServer) createRegexModel(w http.ResponseWriter, r *http.Request) {
	var dto regexModelDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	saved, err := a.store.CreateRegexModel(r.Context(), fromRegexModelDTO(dto))
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.router.ReloadRegexModels(r.Context()); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}
	a.logger.Info("admin regex model created", "id", saved.ID, "pattern", saved.Pattern)
	writeJSON(w, http.StatusOK, toRegexModelDTO(saved))
}

func (a *AdminServer) updateRegexModel(w http.ResponseWriter, r *http.Request) {
	var dto regexModelDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	dto.ID = r.PathValue("id")
	saved, err := a.store.UpdateRegexModel(r.Context(), fromRegexModelDTO(dto))
	if errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "regex model not found")
		return
	}
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.router.ReloadRegexModels(r.Context()); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRegexModelDTO(saved))
}

// updateRegexModelPriority is the narrow drag-and-drop endpoint. It
// accepts only {"priority": int} so the dashboard can reorder rows
// without round-tripping every other field.
func (a *AdminServer) updateRegexModelPriority(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	id := r.PathValue("id")
	if err := a.store.UpdateRegexModelPriority(r.Context(), id, body.Priority); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "regex model not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.router.ReloadRegexModels(r.Context()); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminServer) deleteRegexModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteRegexModel(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "regex model not found")
		return
	} else if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.router.ReloadRegexModels(r.Context()); err != nil {
		a.logger.Warn("router reload after regex model delete failed", "id", id, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- resolve tester ----------------------------------------------------

// resolveModel is the dashboard's "what would happen if I sent model X
// right now?" probe. It runs the same pre-resolver as the data plane
// but returns the full trace so the operator can see exactly which
// virtual model / regex model fired and what the eventual upstream
// target would be. The endpoint never makes a real upstream request,
// so it is safe to call repeatedly while editing rules.
func (a *AdminServer) resolveModel(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if model == "" {
		writeAdminError(w, http.StatusBadRequest, "model query parameter is required")
		return
	}
	target, trace, err := a.router.ResolveModel(model)
	resp := map[string]any{
		"input": model,
		"trace": trace,
	}
	if err != nil {
		resp["error"] = err.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if target != nil {
		resp["target"] = map[string]string{
			"provider": target.Provider,
			"model":    target.Model,
		}
		resp["passthrough"] = false
	} else {
		resp["passthrough"] = true
	}
	writeJSON(w, http.StatusOK, resp)
}

// reloadResolverState is a helper used by /admin/reload to refresh the
// resolver tables alongside the legacy provider/route tables. Pulled
// out so the existing reload handler can call it without growing too
// large.
func (a *AdminServer) reloadResolverState(ctx context.Context) error {
	if err := a.router.ReloadVirtualModels(ctx); err != nil {
		return err
	}
	return a.router.ReloadRegexModels(ctx)
}
