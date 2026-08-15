// Package security implements the Security module from DESIGN.md 7.3:
// DLP rules that identify sensitive content in outbound request text and
// mask or block it. Checksummed types (ID card, bank card) are validated
// with their real check-digit algorithms, not just a regex shape match,
// to cut down on false positives from things that merely look like a
// number of the right length.
//
// v1 only ever scans request content, never the model's response -- see
// service.go for why.
package security

import "time"

type MatchType string

const (
	MatchTypeRegexChecksum MatchType = "regex_checksum"
	MatchTypeKeyword       MatchType = "keyword"
)

type RuleAction string

const (
	ActionMask  RuleAction = "mask"
	ActionBlock RuleAction = "block"
)

type DLPRule struct {
	ID        string
	Name      string
	MatchType MatchType
	Pattern   string
	Action    RuleAction
	Priority  int
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SecurityEvent struct {
	ID           string
	MemberID     *string
	VirtualKeyID *string
	RuleID       *string
	Description  string
	ActionTaken  RuleAction
	OccurredAt   time.Time
}
