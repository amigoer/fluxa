// virtual_models.go is the persistence layer for the v2.4 alias and
// traffic-splitting feature. A virtual model is a stable user-facing
// name (e.g. "qwen-latest") that fans out to one or more concrete
// targets, each carrying a positive integer weight that drives a
// random pick at request time. The targets themselves can be either
// real models (resolved through the existing routes/providers tables)
// or other virtual models (resolved recursively, with a depth cap
// enforced in internal/router/model_resolver.go).
//
// The two-table layout (virtual_models + virtual_model_routes) keeps
// individual route edits cheap and lets ON DELETE CASCADE clean up
// children when a parent is deleted. We persist a position column on
// each child so the dashboard can preserve operator-chosen ordering;
// internally the resolver only cares about weights, but a stable
// list order matters for the UI's "drag-and-drop to reorder" affordance.

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// VirtualModel is the parent row. Routes is populated by ListVirtualModels
// / GetVirtualModel via a single LEFT JOIN; callers that only need the
// header (e.g. listing names for a dropdown) can ignore the slice.
type VirtualModel struct {
	ID          string
	Name        string
	Description string
	Enabled     bool
	Routes      []VirtualModelRoute
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// VirtualModelRoute is one weighted target inside a virtual model. The
// resolver only picks routes with Enabled = true; disabled rows stay
// in the table so an operator can flip them back on without losing
// the weight value.
type VirtualModelRoute struct {
	ID          string
	Weight      int
	TargetType  string // "real" | "virtual"
	TargetModel string
	Provider    string // required when TargetType == "real"
	Enabled     bool
	Position    int
}

// newID returns a 16-hex-character random identifier. We use
// crypto/rand instead of math/rand because admin row IDs end up in
// audit logs and external references; predictability would let an
// attacker guess sibling rows.
func newID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

// ListVirtualModels returns every virtual model with its routes
// attached. The query joins on virtual_model_routes so a single round
// trip pulls the full graph; callers do not need to issue follow-up
// queries per parent. Routes are emitted in (parent, position) order
// so list and detail responses are byte-stable across calls.
func (s *Store) ListVirtualModels(ctx context.Context) ([]VirtualModel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vm.id, vm.name, vm.description, vm.enabled, vm.created_at, vm.updated_at,
		       vmr.id, vmr.weight, vmr.target_type, vmr.target_model, vmr.provider, vmr.enabled, vmr.position
		FROM virtual_models vm
		LEFT JOIN virtual_model_routes vmr ON vmr.virtual_model_id = vm.id
		ORDER BY vm.name, vmr.position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// We stream the join into an ordered slice + index map so each
	// parent shows up exactly once with its routes appended in order.
	var (
		out []VirtualModel
		idx = make(map[string]int)
	)
	for rows.Next() {
		var (
			vm           VirtualModel
			enabledInt   int
			rID          sql.NullString
			rWeight      sql.NullInt64
			rTargetType  sql.NullString
			rTargetModel sql.NullString
			rProvider    sql.NullString
			rEnabled     sql.NullInt64
			rPosition    sql.NullInt64
		)
		if err := rows.Scan(
			&vm.ID, &vm.Name, &vm.Description, &enabledInt, &vm.CreatedAt, &vm.UpdatedAt,
			&rID, &rWeight, &rTargetType, &rTargetModel, &rProvider, &rEnabled, &rPosition,
		); err != nil {
			return nil, err
		}
		vm.Enabled = enabledInt == 1
		i, ok := idx[vm.ID]
		if !ok {
			out = append(out, vm)
			i = len(out) - 1
			idx[vm.ID] = i
		}
		if rID.Valid {
			out[i].Routes = append(out[i].Routes, VirtualModelRoute{
				ID:          rID.String,
				Weight:      int(rWeight.Int64),
				TargetType:  rTargetType.String,
				TargetModel: rTargetModel.String,
				Provider:    rProvider.String,
				Enabled:     rEnabled.Int64 == 1,
				Position:    int(rPosition.Int64),
			})
		}
	}
	return out, rows.Err()
}

// GetVirtualModel loads a single virtual model by name. Returns
// ErrNotFound if no parent matches; an existing parent with no routes
// returns successfully with an empty Routes slice (the router treats
// that as "unresolvable" but the store layer stays neutral).
func (s *Store) GetVirtualModel(ctx context.Context, name string) (VirtualModel, error) {
	var (
		vm         VirtualModel
		enabledInt int
	)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM virtual_models WHERE name = ?`, name)
	if err := row.Scan(&vm.ID, &vm.Name, &vm.Description, &enabledInt, &vm.CreatedAt, &vm.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VirtualModel{}, ErrNotFound
		}
		return VirtualModel{}, err
	}
	vm.Enabled = enabledInt == 1

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, weight, target_type, target_model, provider, enabled, position
		FROM virtual_model_routes WHERE virtual_model_id = ?
		ORDER BY position`, vm.ID)
	if err != nil {
		return VirtualModel{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			r          VirtualModelRoute
			enabledInt int
		)
		if err := rows.Scan(&r.ID, &r.Weight, &r.TargetType, &r.TargetModel, &r.Provider, &enabledInt, &r.Position); err != nil {
			return VirtualModel{}, err
		}
		r.Enabled = enabledInt == 1
		vm.Routes = append(vm.Routes, r)
	}
	return vm, rows.Err()
}

// UpsertVirtualModel inserts or fully replaces a virtual model and its
// routes in a single transaction. "Full replace" matches the REST
// PUT semantics described in the spec: the caller sends the desired
// final state and the store reconciles by deleting all existing
// children and reinserting from the input. This is simpler than a
// per-row diff and the row counts are tiny (a virtual model with 100
// routes is already pathological). The transaction means no partial
// state ever leaks if the function returns an error.
func (s *Store) UpsertVirtualModel(ctx context.Context, vm VirtualModel) (VirtualModel, error) {
	if vm.Name == "" {
		return VirtualModel{}, errors.New("store: virtual_model.name is required")
	}
	if len(vm.Routes) == 0 {
		return VirtualModel{}, errors.New("store: virtual_model requires at least one route")
	}
	for i, rt := range vm.Routes {
		if rt.Weight <= 0 {
			return VirtualModel{}, fmt.Errorf("store: virtual_model.routes[%d].weight must be > 0", i)
		}
		if rt.TargetType != "real" && rt.TargetType != "virtual" {
			return VirtualModel{}, fmt.Errorf("store: virtual_model.routes[%d].target_type must be 'real' or 'virtual'", i)
		}
		if rt.TargetModel == "" {
			return VirtualModel{}, fmt.Errorf("store: virtual_model.routes[%d].target_model is required", i)
		}
		if rt.TargetType == "real" && rt.Provider == "" {
			return VirtualModel{}, fmt.Errorf("store: virtual_model.routes[%d].provider is required when target_type='real'", i)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return VirtualModel{}, err
	}
	defer tx.Rollback()

	// We treat the unique name as the natural key for upsert. If a
	// row already exists we keep its id (so external references stay
	// stable) and bump updated_at; otherwise we mint a new id.
	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM virtual_models WHERE name = ?`, vm.Name).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return VirtualModel{}, err
	}
	if existingID == "" {
		vm.ID = newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO virtual_models (id, name, description, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			vm.ID, vm.Name, vm.Description, boolToInt(vm.Enabled)); err != nil {
			return VirtualModel{}, err
		}
	} else {
		vm.ID = existingID
		if _, err := tx.ExecContext(ctx, `
			UPDATE virtual_models
			SET description = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`, vm.Description, boolToInt(vm.Enabled), vm.ID); err != nil {
			return VirtualModel{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM virtual_model_routes WHERE virtual_model_id = ?`, vm.ID); err != nil {
			return VirtualModel{}, err
		}
	}

	for i := range vm.Routes {
		vm.Routes[i].ID = newID()
		vm.Routes[i].Position = i
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO virtual_model_routes (
				id, virtual_model_id, weight, target_type, target_model, provider, enabled, position
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			vm.Routes[i].ID, vm.ID, vm.Routes[i].Weight, vm.Routes[i].TargetType,
			vm.Routes[i].TargetModel, vm.Routes[i].Provider, boolToInt(vm.Routes[i].Enabled),
			vm.Routes[i].Position); err != nil {
			return VirtualModel{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return VirtualModel{}, err
	}
	// Re-read so timestamps reflect what the database actually wrote
	// rather than what the caller passed in.
	return s.GetVirtualModel(ctx, vm.Name)
}

// DeleteVirtualModel removes a virtual model and (via ON DELETE
// CASCADE) all of its child routes.
func (s *Store) DeleteVirtualModel(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM virtual_models WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
