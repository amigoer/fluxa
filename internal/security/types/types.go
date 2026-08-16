// Package types holds the Security module's domain entities. The module
// implements DESIGN.md 7.3: DLP rules that identify sensitive content in
// outbound request text and mask or block it. Checksummed kinds (ID
// card, bank card) are validated with their real check-digit algorithms,
// not just a regex shape match, to cut down on false positives from
// things that merely look like a number of the right length.
//
// v1 only ever scans request content, never the model's response -- see
// the service package for why.
//
// It imports nothing else from this module, so security/repo,
// security/service and security/handler can all depend on it without an
// import cycle.
package types

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
