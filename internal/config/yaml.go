// yaml.go — YAML import/export bundle for provider and route state.
//
// The gateway no longer boots from a YAML file, but operators still
// want a one-file snapshot format for backup, diffing and bulk edits.
// The /admin/config/export endpoint marshals the store's current
// providers + routes into a Bundle using this schema; /admin/config/
// import parses the same bytes back. The shape intentionally mirrors
// the v1.x config file so existing fluxa.yaml documents round-trip
// unchanged.
//
// Environment variable expansion (${VAR} / ${VAR:-default}) is still
// honoured on import so operators can keep secrets out of the YAML
// and injected at runtime.

package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Bundle is the YAML document shape produced by export and consumed
// by import. Only providers and routes live inside — server /
// database / logging are owned by env vars now.
type Bundle struct {
	Providers []ProviderConfig `yaml:"providers"`
	Routes    []RouteConfig    `yaml:"routes"`
}

// Marshal serialises providers and routes as a YAML Bundle.
func Marshal(providers []ProviderConfig, routes []RouteConfig) ([]byte, error) {
	bundle := Bundle{Providers: providers, Routes: routes}
	return yaml.Marshal(bundle)
}

// Unmarshal decodes YAML bytes into a Bundle, runs env expansion on
// the raw source first, and validates the result. Callers get a
// consistent, ready-to-persist pair of slices or a descriptive error.
func Unmarshal(raw []byte) (Bundle, error) {
	expanded := expandEnv(string(raw))
	var b Bundle
	if err := yaml.Unmarshal([]byte(expanded), &b); err != nil {
		return Bundle{}, fmt.Errorf("parse yaml: %w", err)
	}
	b.applyDefaults()
	if err := b.Validate(); err != nil {
		return Bundle{}, err
	}
	return b, nil
}

// applyDefaults fills per-provider defaults so importers don't have
// to repeat the kind field when it matches the provider name.
func (b *Bundle) applyDefaults() {
	for i := range b.Providers {
		p := &b.Providers[i]
		if p.Kind == "" {
			p.Kind = p.Name
		}
	}
}

// Validate returns the first schema error discovered in the bundle.
// It enforces the basic invariants router.Reload would otherwise
// complain about, but in a friendlier way so bad imports are
// rejected before they reach the store.
func (b *Bundle) Validate() error {
	seen := make(map[string]struct{}, len(b.Providers))
	for _, p := range b.Providers {
		if p.Name == "" {
			return errors.New("provider.name is required")
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("duplicate provider name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
	}
	for _, r := range b.Routes {
		if r.Model == "" {
			return errors.New("route.model is required")
		}
		if _, ok := seen[r.Provider]; !ok {
			return fmt.Errorf("route %q references unknown provider %q", r.Model, r.Provider)
		}
		for _, fb := range r.Fallback {
			if _, ok := seen[fb]; !ok {
				return fmt.Errorf("route %q fallback references unknown provider %q", r.Model, fb)
			}
		}
	}
	return nil
}

// envPattern matches ${VAR} and ${VAR:-default} placeholders.
var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandEnv replaces ${VAR} placeholders inside the YAML source with
// their environment values. Unknown variables expand to the empty
// string unless a ":-default" clause is supplied.
func expandEnv(src string) string {
	return envPattern.ReplaceAllStringFunc(src, func(match string) string {
		groups := envPattern.FindStringSubmatch(match)
		if v, ok := os.LookupEnv(groups[1]); ok {
			return v
		}
		return groups[2]
	})
}
