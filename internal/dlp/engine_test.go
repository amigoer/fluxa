package dlp

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
)

func compileRules(rules ...*CompiledRule) []*CompiledRule { return rules }

func kw(id, name, keyword, scope, action string, priority int) *CompiledRule {
	return &CompiledRule{
		ID:          id,
		Name:        name,
		Pattern:     regexp.MustCompile("(?i)" + regexp.QuoteMeta(keyword)),
		PatternRaw:  keyword,
		PatternType: "keyword",
		Scope:       scope,
		Action:      action,
		Priority:    priority,
	}
}

func rx(id, name, pattern, scope, action string, priority int) *CompiledRule {
	return &CompiledRule{
		ID:          id,
		Name:        name,
		Pattern:     regexp.MustCompile(pattern),
		PatternRaw:  pattern,
		PatternType: "regex",
		Scope:       scope,
		Action:      action,
		Priority:    priority,
	}
}

func TestScanRequest_KeywordMatch(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "block-password", "password", "request", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"my password is secret123"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected block, got %q", result.Action)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].MatchedText != "password" {
		t.Errorf("unexpected matched text: %q", result.Violations[0].MatchedText)
	}
}

func TestScanRequest_RegexMatch(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		rx("1", "ssn-detect", `\d{3}-\d{2}-\d{4}`, "request", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"SSN is 123-45-6789"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected block, got %q", result.Action)
	}
	if result.Violations[0].MatchedText != "123-45-6789" {
		t.Errorf("unexpected matched: %q", result.Violations[0].MatchedText)
	}
}

func TestScanRequest_NoMatch(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "block-secret", "secret_key", "request", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"hello world"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "" {
		t.Errorf("expected no action, got %q", result.Action)
	}
}

func TestScanRequest_ScopeFiltering(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "response-only", "password", "response", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"my password"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "" {
		t.Errorf("expected no action (rule is response-only), got %q", result.Action)
	}
}

func TestScanRequest_PriorityOrdering(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "log-first", "password", "request", "log", 10),
		kw("2", "block-second", "password", "request", "block", 50),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"my password"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "log" {
		t.Errorf("expected log (lower priority wins), got %q", result.Action)
	}
	if len(result.Violations) != 2 {
		t.Errorf("expected 2 violations (both matched), got %d", len(result.Violations))
	}
}

func TestScanRequest_ModelPatternFilter(t *testing.T) {
	rule := kw("1", "gpt-only", "password", "request", "block", 10)
	rule.ModelPattern = regexp.MustCompile(`^gpt-`)
	rule.ModelPatRaw = `^gpt-`

	e := &Engine{}
	e.rules = compileRules(rule)

	body := json.RawMessage(`{"model":"claude-3","messages":[{"role":"user","content":"my password"}]}`)
	result := e.ScanRequest(context.Background(), body, "claude-3", "")
	if result.Action != "" {
		t.Errorf("expected no match (model filter), got %q", result.Action)
	}

	result = e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected block for gpt-4, got %q", result.Action)
	}
}

func TestScanRequest_Mask(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		rx("1", "mask-ssn", `\d{3}-\d{2}-\d{4}`, "request", "mask", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"SSN is 123-45-6789 ok"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "mask" {
		t.Fatalf("expected mask, got %q", result.Action)
	}
	if result.Masked == nil {
		t.Fatal("expected masked body")
	}
	// Verify the masked body no longer contains the SSN.
	if matched, _ := regexp.MatchString(`\d{3}-\d{2}-\d{4}`, string(result.Masked)); matched {
		t.Errorf("masked body still contains SSN: %s", result.Masked)
	}
	// Verify *** is present.
	if matched, _ := regexp.MatchString(`\*\*\*`, string(result.Masked)); !matched {
		t.Errorf("masked body doesn't contain ***: %s", result.Masked)
	}
}

func TestScanResponse_OpenAI(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "block-password", "password", "response", "block", 10),
	)
	body := json.RawMessage(`{"choices":[{"message":{"role":"assistant","content":"your password is abc123"}}]}`)
	result := e.ScanResponse(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected block, got %q", result.Action)
	}
}

func TestScanResponse_Anthropic(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "block-password", "password", "response", "block", 10),
	)
	body := json.RawMessage(`{"content":[{"type":"text","text":"your password is abc123"}]}`)
	result := e.ScanResponse(context.Background(), body, "claude-3", "")
	if result.Action != "block" {
		t.Errorf("expected block, got %q", result.Action)
	}
}

func TestScanRequest_MultipartContent(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "block-secret", "secret_key", "request", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"my secret_key is here"},{"type":"image_url","image_url":{"url":"data:..."}}]}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected block for multipart content, got %q", result.Action)
	}
}

func TestScanRequest_BothScope(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "both-scope", "password", "both", "log", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"password"}]}`)

	req := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if req.Action != "log" {
		t.Errorf("expected log for request, got %q", req.Action)
	}

	respBody := json.RawMessage(`{"choices":[{"message":{"content":"password"}}]}`)
	resp := e.ScanResponse(context.Background(), respBody, "gpt-4", "")
	if resp.Action != "log" {
		t.Errorf("expected log for response, got %q", resp.Action)
	}
}

func TestScanRequest_KeywordCaseInsensitive(t *testing.T) {
	e := &Engine{}
	e.rules = compileRules(
		kw("1", "case-test", "Password", "request", "block", 10),
	)
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"MY PASSWORD IS HERE"}]}`)
	result := e.ScanRequest(context.Background(), body, "gpt-4", "")
	if result.Action != "block" {
		t.Errorf("expected case-insensitive match, got %q", result.Action)
	}
}

func TestExtractContent_String(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	got := extractContent(raw)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestExtractContent_Array(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello"},{"type":"image_url","image_url":{}}]`)
	got := extractContent(raw)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestExtractContent_Empty(t *testing.T) {
	if extractContent(nil) != "" {
		t.Error("expected empty for nil")
	}
	if extractContent(json.RawMessage("null")) != "" {
		t.Error("expected empty for null")
	}
}
