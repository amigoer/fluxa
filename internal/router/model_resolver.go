// model_resolver.go is the v2.4 pre-routing pipeline. Every request
// runs through ResolveModel before the legacy Resolve lookup; if the
// pre-stage returns a concrete target, the request handler bypasses
// the legacy chain entirely. The resolver is intentionally
// self-contained — it never mutates router state and never touches
// the database — so the request path stays a tight loop of map
// lookups and (at most) a handful of regex matches.
//
// Resolution order on every call:
//
//  1. exact match in virtual_models? → weighted pick → recurse
//  2. regex match in regex_routes (priority ASC, first wins)? → recurse
//  3. passthrough — return nil and let the caller fall through to
//     the legacy Resolve() chain
//
// The recursion cap (maxResolveDepth) protects against operator
// mistakes that point a virtual model at itself (or at another
// virtual model that points back). When the cap is exceeded the
// resolver returns an error rather than silently looping.

package router

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// maxResolveDepth caps the number of virtual → virtual hops the
// resolver will follow before bailing out. Five is generous: a
// well-formed alias graph rarely needs more than two levels (a
// regex_route pointing at a virtual_model that picks among real
// models). Anything deeper is almost certainly a misconfigured cycle.
const maxResolveDepth = 5

// ErrResolveDepth signals that ResolveModel hit maxResolveDepth.
// Wrapped so callers can distinguish "this is a configuration cycle"
// from "this model is unknown".
var ErrResolveDepth = errors.New("router: model resolve depth exceeded (likely cycle)")

// ResolvedTarget is the value ResolveModel returns when the
// pre-resolver matched something. Provider and Model are the values
// the request handler should use; an empty Provider means "look up
// Model with the legacy Resolve() chain after all" (used by virtual
// targets that resolve to a real model name without naming a
// provider — those still need the legacy fallback chain to apply).
type ResolvedTarget struct {
	Provider string
	Model    string
}

// TraceStep records one decision the resolver made. The slice grows
// in evaluation order so the dashboard's Resolve Tester can render
// the chain top-to-bottom and the operator can see exactly why the
// resolver picked the target it did.
type TraceStep struct {
	Depth        int    `json:"depth"`
	Type         string `json:"type"` // "regex_match" | "virtual_model" | "passthrough"
	Pattern      string `json:"pattern,omitempty"`
	Name         string `json:"name,omitempty"`
	TargetType   string `json:"target_type,omitempty"`
	Target       string `json:"target,omitempty"`
	Provider     string `json:"provider,omitempty"`
	WeightPicked int    `json:"weight_picked,omitempty"`
}

// resolveCtx threads request-scoped state through the recursive
// resolver without polluting the public API. trace is appended to as
// the resolver descends so callers (especially the debug endpoint)
// can see every decision in order. depth tracks recursion to enforce
// maxResolveDepth.
type resolveCtx struct {
	trace []TraceStep
	depth int
}

// ResolveModel is the public entry point invoked from the chat /
// messages handlers. The (target, trace, error) tuple is shaped for
// easy reuse by the /admin/resolve-model debug endpoint: production
// callers can ignore the trace, but it costs nothing to build (the
// slice stays nil unless the resolver actually descends into a
// virtual or regex match).
//
// Returns:
//
//   - target = nil, err = nil → passthrough; the caller should fall
//     through to the legacy Resolve() chain
//   - target != nil, err = nil → use target.Provider + target.Model
//     directly; skip the legacy chain
//   - err != nil → resolution failed (bad target reference, depth
//     exceeded, etc.); the caller should propagate as a 4xx
func (r *Router) ResolveModel(modelName string) (*ResolvedTarget, []TraceStep, error) {
	rc := &resolveCtx{}
	target, err := r.resolveWithCtx(modelName, rc)
	return target, rc.trace, err
}

// resolveWithCtx is the recursive worker. It walks the v2.4 tables
// for the input name, recurses through any virtual → virtual chain,
// and either returns a concrete target or signals passthrough by
// returning (nil, nil).
func (r *Router) resolveWithCtx(name string, rc *resolveCtx) (*ResolvedTarget, error) {
	if rc.depth >= maxResolveDepth {
		return nil, fmt.Errorf("%w: stopped at %q after %d hops", ErrResolveDepth, name, rc.depth)
	}
	s := r.snapshot()

	// Step 1: exact virtual model match.
	if vm, ok := s.virtualModels[name]; ok {
		picked := weightedPick(vm.Routes)
		if picked == nil {
			return nil, fmt.Errorf("router: virtual model %q has no enabled routes", name)
		}
		rc.trace = append(rc.trace, TraceStep{
			Depth:        rc.depth,
			Type:         "virtual_model",
			Name:         vm.Name,
			TargetType:   picked.TargetType,
			Target:       picked.TargetModel,
			Provider:     picked.Provider,
			WeightPicked: picked.Weight,
		})
		return r.followTarget(picked.TargetType, picked.TargetModel, picked.Provider, rc)
	}

	// Step 2: regex match. The slice is pre-sorted by priority so we
	// can take the first hit and stop scanning.
	for _, rr := range s.regexRoutes {
		if !rr.Pattern.MatchString(name) {
			continue
		}
		rc.trace = append(rc.trace, TraceStep{
			Depth:      rc.depth,
			Type:       "regex_match",
			Pattern:    rr.PatternRaw,
			TargetType: rr.TargetType,
			Target:     rr.TargetModel,
			Provider:   rr.Provider,
		})
		return r.followTarget(rr.TargetType, rr.TargetModel, rr.Provider, rc)
	}

	// Step 3: passthrough. Record it in the trace so the debug
	// endpoint can show that the resolver evaluated everything and
	// declined to intervene; the request will then resolve through
	// the legacy chain.
	rc.trace = append(rc.trace, TraceStep{
		Depth:  rc.depth,
		Type:   "passthrough",
		Target: name,
	})
	return nil, nil
}

// followTarget interprets a (target_type, target_model, provider)
// triple from a virtual model route or a regex route. Real targets
// resolve immediately to a ResolvedTarget; virtual targets recurse
// after bumping the depth counter.
func (r *Router) followTarget(targetType, target, provider string, rc *resolveCtx) (*ResolvedTarget, error) {
	switch targetType {
	case "real":
		return &ResolvedTarget{Provider: provider, Model: target}, nil
	case "virtual":
		rc.depth++
		next, err := r.resolveWithCtx(target, rc)
		if err != nil {
			return nil, err
		}
		if next == nil {
			// Recursing into a virtual that itself passes through is
			// almost certainly an operator typo: the inner virtual
			// model name does not exist (or has no enabled routes
			// and no regex match either). Surface it explicitly
			// instead of letting the request fall through to legacy
			// resolve and 404 there with a confusing error.
			return nil, fmt.Errorf("router: virtual target %q does not resolve", target)
		}
		return next, nil
	default:
		return nil, fmt.Errorf("router: unknown target_type %q", targetType)
	}
}

// weightedPick selects one route from the slice with probability
// proportional to its Weight. Modulo of the weight sum would skew the
// distribution; using rand.Intn(total) and walking the cumulative sum
// gives an exact distribution. Returns nil only if the slice is
// empty (the caller should treat that as a configuration error).
func weightedPick(routes []VirtualModelRoute) *VirtualModelRoute {
	if len(routes) == 0 {
		return nil
	}
	total := 0
	for i := range routes {
		total += routes[i].Weight
	}
	if total <= 0 {
		return nil
	}
	n := resolverRand().Intn(total)
	for i := range routes {
		n -= routes[i].Weight
		if n < 0 {
			return &routes[i]
		}
	}
	return &routes[len(routes)-1]
}

// resolverRand lazily constructs a per-process *rand.Rand seeded
// from the wall clock. We do this rather than using the package-level
// math/rand so tests can override the source via SetResolverRand for
// deterministic weighted-pick assertions.
var (
	rngOnce sync.Once
	rng     *rand.Rand
	rngMu   sync.Mutex
)

func resolverRand() *rand.Rand {
	rngOnce.Do(func() {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
	rngMu.Lock()
	defer rngMu.Unlock()
	return rng
}

// SetResolverRand replaces the resolver's random source. Tests use
// this to make weighted picks deterministic; production code never
// touches it.
func SetResolverRand(r *rand.Rand) {
	rngOnce.Do(func() {}) // mark as initialised so the lazy ctor does not overwrite
	rngMu.Lock()
	rng = r
	rngMu.Unlock()
}
