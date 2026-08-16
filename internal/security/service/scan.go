package service

import (
	"context"
	"strings"

	"github.com/amigoer/fluxa/internal/security/rules"
	"github.com/amigoer/fluxa/internal/security/types"
)

// ScanService is the DLP check the gateway runs on every request.
type ScanService interface {
	Scan(ctx context.Context, text string) (ScanResult, error)
}

// Hit records that one rule matched during a Scan, without keeping the
// actual sensitive substring around: types.SecurityEvent.Description is a
// fixed, generic phrase (e.g. "身份证号已脱敏", matching the mockup),
// never the value itself, so the audit trail doesn't become a second
// place sensitive data leaks from.
type Hit struct {
	Rule types.DLPRule
}

// ScanResult is what Service.Scan found in one piece of request text.
type ScanResult struct {
	// MaskedText is text with every mask-action match replaced. If
	// Blocked is true, MaskedText should not be used -- the whole
	// request is rejected, not partially redacted.
	MaskedText string
	Blocked    bool
	Hits       []Hit
}

// Scan applies every enabled DLP rule to text, in priority order
// (DESIGN.md 7.3). It only ever looks at outbound request content --
// there is no equivalent call for a model's response. Scanning the
// response would mean either buffering a streaming reply before
// forwarding it (breaking the gateway's "never buffer a streaming
// response" performance rule) or a much more involved sliding-window
// scan across chunk boundaries; v1 accepts the residual risk of a model
// echoing sensitive input back and leaves that to v2.
func (s *service) Scan(ctx context.Context, text string) (ScanResult, error) {
	enabledRules, err := s.repo.ListEnabledRulesByPriority(ctx)
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{MaskedText: text}

	for _, rule := range enabledRules {
		matches := findMatches(rule, result.MaskedText)
		if len(matches) == 0 {
			continue
		}

		result.Hits = append(result.Hits, Hit{Rule: rule})

		if rule.Action == types.ActionBlock {
			result.Blocked = true
			return result, nil
		}

		for _, m := range matches {
			result.MaskedText = strings.ReplaceAll(result.MaskedText, m, maskValue(m))
		}
	}

	return result, nil
}

// findMatches dispatches a rule to its detector. For regex_checksum
// rules, Pattern names one of the built-in, checksum-validated types
// (id_card / bank_card / phone) rather than holding an arbitrary regex
// -- those three are the only checksummed types v1 ships (DESIGN.md
// 7.3: international types like passport/SSN are an intentionally
// unimplemented extension point). For keyword rules, Pattern is a
// comma-separated keyword list.
func findMatches(rule types.DLPRule, text string) []string {
	switch rule.MatchType {
	case types.MatchTypeRegexChecksum:
		switch rule.Pattern {
		case "id_card":
			return rules.FindIDCards(text)
		case "bank_card":
			return rules.FindBankCards(text)
		case "phone":
			return rules.FindPhoneNumbers(text)
		}
		return nil
	case types.MatchTypeKeyword:
		var found []string
		for _, kw := range strings.Split(rule.Pattern, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(text, kw) {
				found = append(found, kw)
			}
		}
		return found
	default:
		return nil
	}
}

// maskValue redacts a matched value, keeping its first and last
// character so a human glancing at a log can still tell what type of
// thing was there without the value itself being recoverable.
func maskValue(value string) string {
	runes := []rune(value)
	if len(runes) <= 2 {
		return strings.Repeat("*", len(runes))
	}
	masked := make([]rune, len(runes))
	masked[0] = runes[0]
	masked[len(runes)-1] = runes[len(runes)-1]
	for i := 1; i < len(runes)-1; i++ {
		masked[i] = '*'
	}
	return string(masked)
}
