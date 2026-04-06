// Package router owns the mapping from model identifiers to the ordered list
// of providers that can serve them. It also instantiates concrete adapters
// from the declarative configuration so the rest of the codebase never has to
// know which provider kind is in play.
package router

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/amigoer/fluxa/internal/adapter/anthropic"
	"github.com/amigoer/fluxa/internal/adapter/azure"
	"github.com/amigoer/fluxa/internal/adapter/gemini"
	"github.com/amigoer/fluxa/internal/adapter/openai"
	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/provider"
)

// Router resolves a model to an ordered slice of Providers (primary then
// fallbacks). It is immutable once built and safe for concurrent reads.
type Router struct {
	providers map[string]provider.Provider
	routes    map[string][]string // model -> ordered provider names
	// catchAll is the default provider name used when a request arrives for
	// a model that has no explicit route. When empty, unknown models are
	// rejected. Configured via the first route with model "*".
	catchAll []string
}

// Build constructs a Router from a validated Config. It instantiates one
// Provider per entry in cfg.Providers and wires the route table.
func Build(cfg config.Config) (*Router, error) {
	providers := make(map[string]provider.Provider, len(cfg.Providers))
	for _, pc := range cfg.Providers {
		p, err := newProvider(pc)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", pc.Name, err)
		}
		providers[pc.Name] = p
	}

	r := &Router{
		providers: providers,
		routes:    make(map[string][]string, len(cfg.Routes)),
	}
	for _, route := range cfg.Routes {
		chain := append([]string{route.Provider}, route.Fallback...)
		if route.Model == "*" {
			r.catchAll = chain
			continue
		}
		r.routes[route.Model] = chain
	}
	return r, nil
}

// Providers returns a deterministic snapshot of all configured providers. It
// is used by the /health handler to probe connectivity in parallel.
func (r *Router) Providers() map[string]provider.Provider {
	out := make(map[string]provider.Provider, len(r.providers))
	for k, v := range r.providers {
		out[k] = v
	}
	return out
}

// Models returns the aggregated set of model identifiers served by the
// gateway. It is used to implement GET /v1/models.
func (r *Router) Models() []string {
	seen := make(map[string]struct{})
	for model := range r.routes {
		seen[model] = struct{}{}
	}
	for _, p := range r.providers {
		for _, m := range p.Models() {
			seen[m] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	return out
}

// ErrUnknownModel is returned when no explicit route and no catch-all cover
// the requested model.
var ErrUnknownModel = errors.New("unknown model")

// Resolve returns the ordered list of providers that can serve the given
// model. The first entry is the primary; subsequent entries are fallbacks.
func (r *Router) Resolve(model string) ([]provider.Provider, error) {
	chain, ok := r.routes[model]
	if !ok {
		// Fall back to any provider that advertises the model through its
		// static Models() list. This keeps small single-provider configs
		// working without requiring an explicit routes section.
		if chain = r.providersServing(model); len(chain) > 0 {
			// no-op, chain is ready below
		} else if len(r.catchAll) > 0 {
			chain = r.catchAll
		} else {
			return nil, fmt.Errorf("%w: %s", ErrUnknownModel, model)
		}
	}
	out := make([]provider.Provider, 0, len(chain))
	for _, name := range chain {
		p, ok := r.providers[name]
		if !ok {
			return nil, fmt.Errorf("router: provider %q not registered", name)
		}
		out = append(out, p)
	}
	return out, nil
}

// providersServing returns the names of providers whose static model list
// contains the given model. Order is non-deterministic so callers that need
// stability should declare an explicit route.
func (r *Router) providersServing(model string) []string {
	var out []string
	for name, p := range r.providers {
		for _, m := range p.Models() {
			if m == model {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

// openaiCompatibleDefaults lists every provider kind that speaks the OpenAI
// REST dialect. Adding support for another Chinese or Western vendor that
// exposes an OpenAI-compatible endpoint is a one-line edit here.
var openaiCompatibleDefaults = map[string]string{
	"openai":   "https://api.openai.com/v1",
	"deepseek": "https://api.deepseek.com/v1",
	"qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"ollama":   "http://localhost:11434/v1",
	// v4.0 — additional providers that ship an OpenAI-compatible surface.
	"moonshot": "https://api.moonshot.cn/v1",        // Kimi
	"zhipu":    "https://open.bigmodel.cn/api/paas/v4", // GLM
	"doubao":   "https://ark.cn-beijing.volces.com/api/v3", // Volcengine Ark
	"ernie":    "https://qianfan.baidubce.com/v2",   // Baidu Qianfan v2
}

// newProvider is the factory from config.ProviderConfig to a concrete
// Provider instance. Unknown kinds return an error so operators find typos
// at startup instead of runtime.
func newProvider(pc config.ProviderConfig) (provider.Provider, error) {
	if base, ok := openaiCompatibleDefaults[pc.Kind]; ok {
		return openai.New(openai.Options{
			Name:    pc.Name,
			BaseURL: defaultBaseURL(pc, base),
			APIKey:  pc.APIKey,
			Models:  pc.Models,
			Headers: pc.Headers,
			Timeout: pc.Timeout,
		}), nil
	}
	switch pc.Kind {
	case "anthropic":
		return anthropic.New(anthropic.Options{
			Name: pc.Name, BaseURL: defaultBaseURL(pc, "https://api.anthropic.com"),
			APIKey: pc.APIKey, Models: pc.Models, Headers: pc.Headers, Timeout: pc.Timeout,
		}), nil
	case "gemini":
		return gemini.New(gemini.Options{
			Name:    pc.Name,
			BaseURL: defaultBaseURL(pc, "https://generativelanguage.googleapis.com/v1beta"),
			APIKey:  pc.APIKey,
			Models:  pc.Models,
			Headers: pc.Headers,
			Timeout: pc.Timeout,
		}), nil
	case "azure":
		return azure.New(azure.Options{
			Name:        pc.Name,
			BaseURL:     pc.BaseURL,
			APIKey:      pc.APIKey,
			APIVersion:  pc.APIVersion,
			Deployments: pc.Deployments,
			Headers:     pc.Headers,
			Timeout:     pc.Timeout,
		})
	default:
		return nil, fmt.Errorf("unknown provider kind %q", pc.Kind)
	}
}

func defaultBaseURL(pc config.ProviderConfig, fallback string) string {
	if pc.BaseURL != "" {
		return pc.BaseURL
	}
	return fallback
}

// HealthReport summarises the per-provider probe status returned by
// CheckHealth. It is consumed by the /health HTTP handler.
type HealthReport struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// CheckHealth probes every provider in parallel and returns a slice of
// HealthReport entries in provider name order.
func (r *Router) CheckHealth(ctx context.Context) []HealthReport {
	reports := make([]HealthReport, 0, len(r.providers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, p := range r.providers {
		wg.Add(1)
		go func(name string, p provider.Provider) {
			defer wg.Done()
			report := HealthReport{Provider: name, OK: true}
			if err := p.Health(ctx); err != nil {
				report.OK = false
				report.Error = err.Error()
			}
			mu.Lock()
			reports = append(reports, report)
			mu.Unlock()
		}(name, p)
	}
	wg.Wait()
	return reports
}
