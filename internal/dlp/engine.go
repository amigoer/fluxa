// Package dlp implements a Data Loss Prevention engine that scans
// request and response content for sensitive data patterns. Rules are
// compiled at reload time (like the router's CompiledRegexModel) and
// stored behind an RWMutex so the request path is a tight loop of
// regex matches with no allocation overhead.
package dlp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/amigoer/fluxa/internal/store"
)

// CompiledRule is one dlp_rules row with its pattern pre-compiled.
type CompiledRule struct {
	ID           string
	Name         string
	Pattern      *regexp.Regexp
	PatternRaw   string
	PatternType  string // "keyword" | "regex"
	Scope        string // "request" | "response" | "both"
	Action       string // "block" | "mask" | "log"
	Priority     int
	ModelPattern *regexp.Regexp // nil = all models
	ModelPatRaw  string
}

// Violation records one pattern match found during scanning.
type Violation struct {
	RuleID      string
	RuleName    string
	MatchedText string
	Action      string
}

// ScanResult is the outcome of scanning a request or response body.
type ScanResult struct {
	// Action is the winning action from the highest-priority matching
	// rule. Empty string means no rule matched.
	Action     string
	Violations []Violation
	// Masked is populated only when Action == "mask". It contains the
	// body with matched patterns replaced by "***".
	Masked json.RawMessage
}

// Engine holds the pre-compiled DLP rules and provides scan methods.
type Engine struct {
	mu     sync.RWMutex
	rules  []*CompiledRule // sorted by priority ASC
	store  *store.Store
	logger *slog.Logger
}

// NewEngine creates a DLP engine. Call Reload before first use.
func NewEngine(st *store.Store, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{store: st, logger: logger}
}

// Reload reads all enabled DLP rules from the store, compiles their
// patterns, sorts by priority, and atomically swaps the in-memory
// snapshot. Rules that fail to compile are skipped with a warning.
func (e *Engine) Reload(ctx context.Context) error {
	if e.store == nil {
		return nil
	}
	rows, err := e.store.ListDLPRules(ctx)
	if err != nil {
		return fmt.Errorf("dlp: load rules: %w", err)
	}
	var out []*CompiledRule
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		// Compile the content pattern. Keywords are wrapped in
		// QuoteMeta for safe literal matching, case-insensitive.
		var pat string
		if row.PatternType == "keyword" {
			pat = "(?i)" + regexp.QuoteMeta(row.Pattern)
		} else {
			pat = row.Pattern
		}
		re, cerr := regexp.Compile(pat)
		if cerr != nil {
			e.logger.Warn("dlp rule compile failed, skipping",
				"id", row.ID, "name", row.Name, "pattern", row.Pattern, "err", cerr)
			continue
		}

		var modelRe *regexp.Regexp
		if row.ModelPattern != "" {
			modelRe, cerr = regexp.Compile(row.ModelPattern)
			if cerr != nil {
				e.logger.Warn("dlp rule model_pattern compile failed, skipping",
					"id", row.ID, "name", row.Name, "model_pattern", row.ModelPattern, "err", cerr)
				continue
			}
		}

		out = append(out, &CompiledRule{
			ID:           row.ID,
			Name:         row.Name,
			Pattern:      re,
			PatternRaw:   row.Pattern,
			PatternType:  row.PatternType,
			Scope:        row.Scope,
			Action:       row.Action,
			Priority:     row.Priority,
			ModelPattern: modelRe,
			ModelPatRaw:  row.ModelPattern,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority < out[j].Priority
	})

	e.mu.Lock()
	e.rules = out
	e.mu.Unlock()
	return nil
}

// snapshot returns the current rules slice under RLock.
func (e *Engine) snapshot() []*CompiledRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rules
}

// ScanRequest scans an inbound request body for DLP violations.
func (e *Engine) ScanRequest(_ context.Context, body json.RawMessage, model, _ string) ScanResult {
	text := extractRequestText(body)
	return e.scan(text, body, model, "request")
}

// ScanResponse scans a provider response body for DLP violations.
func (e *Engine) ScanResponse(_ context.Context, body json.RawMessage, model, _ string) ScanResult {
	text := extractResponseText(body)
	return e.scan(text, body, model, "response")
}

// scan runs every applicable rule against the concatenated text content.
func (e *Engine) scan(text string, body json.RawMessage, model, direction string) ScanResult {
	if text == "" {
		return ScanResult{}
	}
	rules := e.snapshot()
	if len(rules) == 0 {
		return ScanResult{}
	}

	var violations []Violation
	winningAction := ""

	for _, r := range rules {
		// Scope filter.
		if r.Scope != "both" && r.Scope != direction {
			continue
		}
		// Model filter.
		if r.ModelPattern != nil && !r.ModelPattern.MatchString(model) {
			continue
		}
		// Pattern match.
		matched := r.Pattern.FindString(text)
		if matched == "" {
			continue
		}
		// Truncate matched text for the violation record.
		if len(matched) > 200 {
			matched = matched[:200]
		}
		violations = append(violations, Violation{
			RuleID:      r.ID,
			RuleName:    r.Name,
			MatchedText: matched,
			Action:      r.Action,
		})
		// First matching rule's action wins (priority ASC).
		if winningAction == "" {
			winningAction = r.Action
		}
	}

	if len(violations) == 0 {
		return ScanResult{}
	}

	result := ScanResult{
		Action:     winningAction,
		Violations: violations,
	}

	// Build masked body if the winning action is "mask".
	if winningAction == "mask" {
		result.Masked = maskBody(body, rules, model, direction)
	}

	return result
}

// RecordViolations writes violation records to the store. Intended to
// be called asynchronously (in a goroutine) from the request path so
// DB writes do not add latency.
func (e *Engine) RecordViolations(ctx context.Context, violations []Violation, keyID, model, direction string) {
	if e.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, v := range violations {
		if err := e.store.InsertDLPViolation(ctx, store.DLPViolation{
			RuleID:      v.RuleID,
			RuleName:    v.RuleName,
			KeyID:       keyID,
			Model:       model,
			Direction:   direction,
			MatchedText: v.MatchedText,
			ActionTaken: v.Action,
		}); err != nil {
			e.logger.Warn("dlp: record violation failed", "rule", v.RuleName, "err", err)
		}
	}
}

// ---------- content extraction ----------

// extractRequestText pulls all text content from an OpenAI / Anthropic
// compatible chat request body.
func extractRequestText(body json.RawMessage) string {
	var req struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	var parts []string
	for _, m := range req.Messages {
		parts = append(parts, extractContent(m.Content))
	}
	return strings.Join(parts, "\n")
}

// extractResponseText pulls text from OpenAI and Anthropic response
// formats. OpenAI uses choices[].message.content; Anthropic uses
// content[].text at the top level.
func extractResponseText(body json.RawMessage) string {
	// Try OpenAI format first.
	var openai struct {
		Choices []struct {
			Message struct {
				Content    json.RawMessage `json:"content"`
				ToolCalls  []struct {
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &openai); err == nil && len(openai.Choices) > 0 {
		var parts []string
		for _, c := range openai.Choices {
			parts = append(parts, extractContent(c.Message.Content))
			for _, tc := range c.Message.ToolCalls {
				if tc.Function.Arguments != "" {
					parts = append(parts, tc.Function.Arguments)
				}
			}
		}
		return strings.Join(parts, "\n")
	}

	// Try Anthropic format: { content: [{type:"text", text:"..."}] }
	var anthropic struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &anthropic); err == nil && len(anthropic.Content) > 0 {
		var parts []string
		for _, c := range anthropic.Content {
			if c.Type == "text" && c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// extractContent handles the polymorphic content field: it can be a
// plain string or an array of {type:"text", text:"..."} parts.
func extractContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try as plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as array of parts.
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var out []string
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				out = append(out, p.Text)
			}
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// ---------- masking ----------

// maskBody replaces matched patterns in the body's text content fields
// with "***". It preserves all other JSON fields byte-for-byte.
func maskBody(body json.RawMessage, rules []*CompiledRule, model, direction string) json.RawMessage {
	// Determine which rules apply for masking.
	var applicable []*CompiledRule
	for _, r := range rules {
		if r.Scope != "both" && r.Scope != direction {
			continue
		}
		if r.ModelPattern != nil && !r.ModelPattern.MatchString(model) {
			continue
		}
		applicable = append(applicable, r)
	}
	if len(applicable) == 0 {
		return body
	}

	applyMask := func(text string) string {
		for _, r := range applicable {
			text = r.Pattern.ReplaceAllString(text, "***")
		}
		return text
	}

	// Parse the body as a generic map, walk and mask text fields.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}

	// Mask messages (request format).
	if raw, ok := obj["messages"]; ok {
		obj["messages"] = maskMessages(raw, applyMask)
	}

	// Mask choices (OpenAI response format).
	if raw, ok := obj["choices"]; ok {
		obj["choices"] = maskChoices(raw, applyMask)
	}

	// Mask content array (Anthropic response format).
	if raw, ok := obj["content"]; ok {
		// Only if it's an array (not a string at message level).
		if len(raw) > 0 && raw[0] == '[' {
			obj["content"] = maskContentArray(raw, applyMask)
		}
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return out
}

func maskMessages(raw json.RawMessage, mask func(string) string) json.RawMessage {
	var msgs []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return raw
	}
	for i, m := range msgs {
		if c, ok := m["content"]; ok {
			msgs[i]["content"] = maskContentField(c, mask)
		}
	}
	out, _ := json.Marshal(msgs)
	return out
}

func maskChoices(raw json.RawMessage, mask func(string) string) json.RawMessage {
	var choices []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &choices); err != nil {
		return raw
	}
	for i, ch := range choices {
		if msgRaw, ok := ch["message"]; ok {
			var msg map[string]json.RawMessage
			if err := json.Unmarshal(msgRaw, &msg); err == nil {
				if c, ok := msg["content"]; ok {
					msg["content"] = maskContentField(c, mask)
				}
				choices[i]["message"], _ = json.Marshal(msg)
			}
		}
	}
	out, _ := json.Marshal(choices)
	return out
}

// maskContentField handles the polymorphic content: string or array.
func maskContentField(raw json.RawMessage, mask func(string) string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	// Try as string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		out, _ := json.Marshal(mask(s))
		return out
	}
	// Try as array of parts.
	return maskContentArray(raw, mask)
}

func maskContentArray(raw json.RawMessage, mask func(string) string) json.RawMessage {
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return raw
	}
	for i, p := range parts {
		if t, ok := p["text"]; ok {
			var text string
			if json.Unmarshal(t, &text) == nil {
				parts[i]["text"], _ = json.Marshal(mask(text))
			}
		}
	}
	out, _ := json.Marshal(parts)
	return out
}
