// Package router owns the mapping from model identifiers to the ordered list
// of providers that can serve them. It also instantiates concrete adapters
// from the declarative configuration so the rest of the codebase never has to
// know which provider kind is in play.
//
// As of v1.1 the router is hot-reloadable: its provider and route tables
// live behind an RWMutex and can be swapped at runtime by the admin API
// whenever the underlying store changes.
package router

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"sync"

	"github.com/amigoer/fluxa/internal/adapter/anthropic"
	"github.com/amigoer/fluxa/internal/adapter/azure"
	"github.com/amigoer/fluxa/internal/adapter/bedrock"
	"github.com/amigoer/fluxa/internal/adapter/gemini"
	"github.com/amigoer/fluxa/internal/adapter/openai"
	"github.com/amigoer/fluxa/internal/config"
	"github.com/amigoer/fluxa/internal/provider"
	"github.com/amigoer/fluxa/internal/store"
)

// Router resolves a model to an ordered slice of Providers (primary then
// fallbacks). The internal state is protected by an RWMutex so Reload can
// swap the provider and route tables while requests are in flight.
//
// The store + logger fields are optional. They are only used by the
// v2.4 ReloadVirtualModels / ReloadRegexRoutes helpers; older callers
// that just use Reload(providers, routes) work without ever wiring
// them up.
type Router struct {
	mu     sync.RWMutex
	state  *state
	store  *store.Store
	logger *slog.Logger
}

// SetStore wires the persistence layer the v2.4 reload helpers read
// from. Production code calls this once at boot; tests that only
// exercise Reload(providers, routes) can leave it unset.
func (r *Router) SetStore(s *store.Store) { r.store = s }

// SetLogger installs a slog handler for warning emissions during
// reload (e.g. a regex pattern that fails to compile). Defaults to
// the package slog default when unset.
func (r *Router) SetLogger(l *slog.Logger) {
	if l != nil {
		r.logger = l
	}
}

// state is the immutable snapshot swapped in on every Reload. Reads grab a
// pointer under RLock and can then access the maps without further
// synchronisation for the duration of the request.
type state struct {
	providers map[string]provider.Provider
	routes    map[string][]string // model -> ordered provider names
	// catchAll is the default provider name used when a request arrives for
	// a model that has no explicit route. When empty, unknown models are
	// rejected. Configured via the first route with model "*".
	catchAll []string

	// virtualModels and regexRoutes are populated from a separate
	// store reload (see ReloadVirtualModels / ReloadRegexRoutes). The
	// model resolver in model_resolver.go consults them before falling
	// through to the legacy lookup above. Both fields default to nil
	// on a fresh router so the v2.4 features are strictly opt-in: a
	// gateway with no virtual models and no regex routes behaves
	// exactly like v2.3.
	virtualModels map[string]*VirtualModel
	regexRoutes   []*CompiledRegexRoute
}

// VirtualModel is the in-memory shape of a v2.4 virtual model after
// store reload. It carries only the fields the resolver needs — no
// timestamps, no description — to keep request-path allocations
// minimal. The Routes slice already filters out disabled rows; the
// resolver does not have to re-check Enabled per request.
type VirtualModel struct {
	ID     string
	Name   string
	Routes []VirtualModelRoute
}

// VirtualModelRoute is one weighted target inside a VirtualModel.
type VirtualModelRoute struct {
	Weight      int
	TargetType  string // "real" | "virtual"
	TargetModel string
	Provider    string // populated when TargetType == "real"
}

// CompiledRegexRoute is one regex_routes row with its pattern
// pre-compiled. The compile happens at reload time, not in the
// request path, so request-side resolution is a tight loop of
// MatchString calls and nothing else.
type CompiledRegexRoute struct {
	ID          string
	Pattern     *regexp.Regexp
	PatternRaw  string
	Priority    int
	TargetType  string
	TargetModel string
	Provider    string
}

// New returns an empty Router. Callers must invoke Reload before handling
// requests.
func New() *Router { return &Router{state: &state{providers: map[string]provider.Provider{}, routes: map[string][]string{}}} }

// Reload rebuilds the internal provider map and route table from the given
// configuration slices and atomically swaps them in. It is safe to call
// from any goroutine. If construction fails the current state is left
// untouched so a bad admin edit cannot take the gateway down.
func (r *Router) Reload(providers []config.ProviderConfig, routes []config.RouteConfig) error {
	providerMap := make(map[string]provider.Provider, len(providers))
	for _, pc := range providers {
		p, err := newProvider(pc)
		if err != nil {
			return fmt.Errorf("provider %q: %w", pc.Name, err)
		}
		providerMap[pc.Name] = p
	}

	next := &state{
		providers: providerMap,
		routes:    make(map[string][]string, len(routes)),
	}
	for _, route := range routes {
		if _, ok := providerMap[route.Provider]; !ok {
			return fmt.Errorf("route %q references unknown provider %q", route.Model, route.Provider)
		}
		for _, fb := range route.Fallback {
			if _, ok := providerMap[fb]; !ok {
				return fmt.Errorf("route %q fallback references unknown provider %q", route.Model, fb)
			}
		}
		chain := append([]string{route.Provider}, route.Fallback...)
		if route.Model == "*" {
			next.catchAll = chain
			continue
		}
		next.routes[route.Model] = chain
	}

	// Preserve any v2.4 alias/regex tables that were loaded earlier:
	// Reload only knows about the providers/routes config slice, but
	// the virtual model and regex route tables live in their own
	// store helpers and reload independently. Carrying the previous
	// state forward keeps "operator edits a provider" from
	// accidentally clearing the alias table.
	r.mu.Lock()
	if r.state != nil {
		next.virtualModels = r.state.virtualModels
		next.regexRoutes = r.state.regexRoutes
	}
	r.state = next
	r.mu.Unlock()
	return nil
}

// ReloadVirtualModels rebuilds the in-memory virtual model index from
// the store. It is called once at startup and after every admin write
// to the virtual_models / virtual_model_routes tables. The function
// is a no-op if SetStore was never called, which keeps unit tests
// that hand-build a Router happy.
//
// The function copies the previous snapshot's other fields verbatim
// so a virtual-model edit cannot accidentally clear providers,
// routes, or regex_routes loaded by the other reload paths.
func (r *Router) ReloadVirtualModels(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	rows, err := r.store.ListVirtualModels(ctx)
	if err != nil {
		return fmt.Errorf("router: load virtual models: %w", err)
	}
	index := make(map[string]*VirtualModel, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			// A disabled parent is invisible to the resolver but
			// still visible in the dashboard. Skip it here so the
			// resolver does not need to re-check on every request.
			continue
		}
		vm := &VirtualModel{ID: row.ID, Name: row.Name}
		for _, rt := range row.Routes {
			if !rt.Enabled {
				continue
			}
			vm.Routes = append(vm.Routes, VirtualModelRoute{
				Weight:      rt.Weight,
				TargetType:  rt.TargetType,
				TargetModel: rt.TargetModel,
				Provider:    rt.Provider,
			})
		}
		// A parent with zero enabled routes is unresolvable; surface
		// it as missing rather than picking a random nil. The dashboard
		// can render it as a warning.
		if len(vm.Routes) == 0 {
			continue
		}
		index[row.Name] = vm
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.copyStateLocked()
	next.virtualModels = index
	r.state = next
	return nil
}

// ReloadRegexRoutes rebuilds the in-memory regex route slice from the
// store, sorted by priority ASC. Patterns that fail to compile are
// skipped with a warning so a single bad row cannot brick the whole
// router; the dashboard's create endpoint already validates the
// pattern at write time, so this is a defence-in-depth path for
// imported / hand-edited rows.
func (r *Router) ReloadRegexRoutes(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	rows, err := r.store.ListRegexRoutes(ctx)
	if err != nil {
		return fmt.Errorf("router: load regex routes: %w", err)
	}
	out := make([]*CompiledRegexRoute, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		re, cerr := regexp.Compile(row.Pattern)
		if cerr != nil {
			r.warn("regex route compile failed, skipping",
				"id", row.ID, "pattern", row.Pattern, "err", cerr)
			continue
		}
		out = append(out, &CompiledRegexRoute{
			ID:          row.ID,
			Pattern:     re,
			PatternRaw:  row.Pattern,
			Priority:    row.Priority,
			TargetType:  row.TargetType,
			TargetModel: row.TargetModel,
			Provider:    row.Provider,
		})
	}
	// Stable sort so equal-priority rows preserve insertion order
	// from the store (which is created_at ASC).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority < out[j].Priority
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.copyStateLocked()
	next.regexRoutes = out
	r.state = next
	return nil
}

// copyStateLocked returns a shallow clone of the current state. The
// caller must already hold r.mu (write lock). The clone is needed so
// independent reload paths can swap their slice/map without racing
// each other through r.state.
func (r *Router) copyStateLocked() *state {
	if r.state == nil {
		return &state{
			providers: map[string]provider.Provider{},
			routes:    map[string][]string{},
		}
	}
	return &state{
		providers:     r.state.providers,
		routes:        r.state.routes,
		catchAll:      r.state.catchAll,
		virtualModels: r.state.virtualModels,
		regexRoutes:   r.state.regexRoutes,
	}
}

// warn emits a slog warning, falling back to the package default if
// no logger has been wired in. Pulled out so the reload paths stay
// readable.
func (r *Router) warn(msg string, args ...any) {
	logger := r.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn(msg, args...)
}

// snapshot returns the current immutable state. Callers must treat the
// returned value as read-only.
func (r *Router) snapshot() *state {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Providers returns a deterministic snapshot of all configured providers. It
// is used by the /health handler to probe connectivity in parallel.
func (r *Router) Providers() map[string]provider.Provider {
	s := r.snapshot()
	out := make(map[string]provider.Provider, len(s.providers))
	for k, v := range s.providers {
		out[k] = v
	}
	return out
}

// Models returns the aggregated set of model identifiers served by the
// gateway. It is used to implement GET /v1/models.
func (r *Router) Models() []string {
	s := r.snapshot()
	seen := make(map[string]struct{})
	for model := range s.routes {
		seen[model] = struct{}{}
	}
	for _, p := range s.providers {
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
	s := r.snapshot()
	chain, ok := s.routes[model]
	if !ok {
		// Fall back to any provider that advertises the model through its
		// static Models() list. This keeps small single-provider configs
		// working without requiring an explicit routes section.
		if c := s.providersServing(model); len(c) > 0 {
			chain = c
		} else if len(s.catchAll) > 0 {
			chain = s.catchAll
		} else {
			return nil, fmt.Errorf("%w: %s", ErrUnknownModel, model)
		}
	}
	out := make([]provider.Provider, 0, len(chain))
	for _, name := range chain {
		p, ok := s.providers[name]
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
func (s *state) providersServing(model string) []string {
	var out []string
	for name, p := range s.providers {
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
// exposes an OpenAI-compatible endpoint is a one-line edit here — every
// entry below reuses the single internal/adapter/openai implementation.
var openaiCompatibleDefaults = map[string]string{
	// v1.0 — core providers
	"openai":   "https://api.openai.com/v1",
	"deepseek": "https://api.deepseek.com/v1",
	"qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"ollama":   "http://localhost:11434/v1",

	// v4.0 — additional providers that ship an OpenAI-compatible surface.
	"moonshot": "https://api.moonshot.cn/v1",               // Kimi
	"zhipu":    "https://open.bigmodel.cn/api/paas/v4",     // 智谱 GLM
	"doubao":   "https://ark.cn-beijing.volces.com/api/v3", // 豆包 / Volcengine Ark
	"ernie":    "https://qianfan.baidubce.com/v2",          // 文心一言 / Baidu Qianfan v2

	// Western OpenAI-compatible vendors.
	"mistral":    "https://api.mistral.ai/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"xai":        "https://api.x.ai/v1",                  // Grok
	"perplexity": "https://api.perplexity.ai",
	"together":   "https://api.together.xyz/v1",
	"fireworks":  "https://api.fireworks.ai/inference/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"cohere":     "https://api.cohere.ai/compatibility/v1",
	"nvidia":     "https://integrate.api.nvidia.com/v1",

	// Chinese OpenAI-compatible vendors beyond the v4.0 batch.
	"siliconflow": "https://api.siliconflow.cn/v1",        // 硅基流动
	"minimax":     "https://api.minimax.chat/v1",          // MiniMax
	"baichuan":    "https://api.baichuan-ai.com/v1",       // 百川智能
	"stepfun":     "https://api.stepfun.com/v1",           // 阶跃星辰
	"spark":       "https://spark-api-open.xf-yun.com/v1", // 讯飞星火
	"zero-one":    "https://api.lingyiwanwu.com/v1",       // 零一万物 (Yi)
	"tencent":     "https://api.hunyuan.cloud.tencent.com/v1", // 腾讯混元
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
	case "bedrock":
		return bedrock.New(bedrock.Options{
			Name:         pc.Name,
			Region:       pc.Region,
			AccessKey:    pc.AccessKey,
			SecretKey:    pc.SecretKey,
			SessionToken: pc.SessionToken,
			Models:       pc.Models,
			Timeout:      pc.Timeout,
			BaseURL:      pc.BaseURL,
		})
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
	providers := r.Providers()
	reports := make([]HealthReport, 0, len(providers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, p := range providers {
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
