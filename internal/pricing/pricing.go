// Package pricing owns the built-in price table used to compute a USD
// cost estimate for every chat completion. Prices are expressed in USD
// per million tokens because that is the unit every major vendor
// publishes on their docs page, and it keeps the constants readable.
//
// The table is intentionally small and opinionated: it covers the models
// users actually call through Fluxa today. Unknown models return a zero
// cost, which the caller can surface as "cost unavailable" in the UI
// instead of making up a number.
package pricing

import "strings"

// Price holds the per-million-token cost of prompt and completion tokens
// for a single model id.
type Price struct {
	// InputPerMillion is the USD cost of 1,000,000 prompt tokens.
	InputPerMillion float64
	// OutputPerMillion is the USD cost of 1,000,000 completion tokens.
	OutputPerMillion float64
}

// table maps a lowercase model identifier to its published list price.
// Prefix matching (see Lookup) means a single entry can serve variants
// like "gpt-4o-2024-11-20".
var table = map[string]Price{
	// --- OpenAI ---
	"gpt-4o":           {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4o-mini":      {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gpt-4-turbo":      {InputPerMillion: 10.00, OutputPerMillion: 30.00},
	"gpt-4":            {InputPerMillion: 30.00, OutputPerMillion: 60.00},
	"gpt-3.5-turbo":    {InputPerMillion: 0.50, OutputPerMillion: 1.50},
	"o1":               {InputPerMillion: 15.00, OutputPerMillion: 60.00},
	"o1-mini":          {InputPerMillion: 3.00, OutputPerMillion: 12.00},
	"o1-preview":       {InputPerMillion: 15.00, OutputPerMillion: 60.00},
	"o3-mini":          {InputPerMillion: 1.10, OutputPerMillion: 4.40},
	"o3":               {InputPerMillion: 10.00, OutputPerMillion: 40.00},

	// --- Anthropic ---
	"claude-3-5-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-5-haiku":  {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"claude-3-opus":     {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"claude-3-sonnet":   {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-haiku":    {InputPerMillion: 0.25, OutputPerMillion: 1.25},
	"claude-3-7-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00},

	// --- DeepSeek ---
	"deepseek-chat":     {InputPerMillion: 0.27, OutputPerMillion: 1.10},
	"deepseek-reasoner": {InputPerMillion: 0.55, OutputPerMillion: 2.19},

	// --- Qwen ---
	"qwen-max":   {InputPerMillion: 2.80, OutputPerMillion: 8.40},
	"qwen-plus":  {InputPerMillion: 0.56, OutputPerMillion: 1.68},
	"qwen-turbo": {InputPerMillion: 0.42, OutputPerMillion: 0.84},

	// --- Chinese OpenAI-compatible vendors ---
	"moonshot-v1-8k":      {InputPerMillion: 1.68, OutputPerMillion: 1.68},
	"moonshot-v1-32k":     {InputPerMillion: 3.36, OutputPerMillion: 3.36},
	"moonshot-v1-128k":    {InputPerMillion: 8.40, OutputPerMillion: 8.40},
	"kimi-k2":             {InputPerMillion: 0.42, OutputPerMillion: 2.52},
	"glm-4":               {InputPerMillion: 7.00, OutputPerMillion: 7.00},
	"glm-4-flash":         {InputPerMillion: 0.014, OutputPerMillion: 0.014},
	"doubao-pro-32k":      {InputPerMillion: 0.11, OutputPerMillion: 0.28},
	"ernie-4.0-8k":        {InputPerMillion: 16.80, OutputPerMillion: 16.80},
	"ernie-3.5-8k":        {InputPerMillion: 1.68, OutputPerMillion: 1.68},

	// --- Google Gemini ---
	"gemini-1.5-pro":      {InputPerMillion: 1.25, OutputPerMillion: 5.00},
	"gemini-1.5-flash":    {InputPerMillion: 0.075, OutputPerMillion: 0.30},
	"gemini-2.0-flash":    {InputPerMillion: 0.10, OutputPerMillion: 0.40},

	// --- AWS Bedrock (Anthropic list prices apply) ---
	"anthropic.claude-3-5-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"meta.llama3-1-70b":           {InputPerMillion: 0.99, OutputPerMillion: 0.99},

	// --- Western OpenAI-compatible vendors ---
	"mistral-large": {InputPerMillion: 2.00, OutputPerMillion: 6.00},
	"grok-2":        {InputPerMillion: 2.00, OutputPerMillion: 10.00},
	"grok-2-mini":   {InputPerMillion: 0.30, OutputPerMillion: 0.50},
	"command-r-plus": {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"command-r":      {InputPerMillion: 0.15, OutputPerMillion: 0.60},
}

// Lookup finds the price entry for a model id. It first tries an exact
// match, then falls back to the longest prefix match so that dated
// variants like "gpt-4o-2024-11-20" still resolve to the canonical
// "gpt-4o" entry.
func Lookup(model string) (Price, bool) {
	key := strings.ToLower(model)
	if p, ok := table[key]; ok {
		return p, true
	}
	var (
		best    string
		bestOk  bool
		bestVal Price
	)
	for k, v := range table {
		if strings.HasPrefix(key, k) && len(k) > len(best) {
			best = k
			bestVal = v
			bestOk = true
		}
	}
	return bestVal, bestOk
}

// Cost returns the USD cost for a (promptTokens, completionTokens) pair
// for the given model. It returns 0 when the model is not in the table;
// callers should treat that as "unavailable" rather than "free".
func Cost(model string, promptTokens, completionTokens int) float64 {
	p, ok := Lookup(model)
	if !ok {
		return 0
	}
	return float64(promptTokens)*p.InputPerMillion/1_000_000 +
		float64(completionTokens)*p.OutputPerMillion/1_000_000
}
