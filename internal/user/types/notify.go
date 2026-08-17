package types

import "time"

type NotifyChannelKind string

const (
	NotifyChannelSMS   NotifyChannelKind = "sms"
	NotifyChannelEmail NotifyChannelKind = "email"
)

// NotifyChannel is a pluggable sending channel (DESIGN.md 7.1): Config
// holds provider-specific credentials (AccessKey/secret/sign/template
// for SMS, or SMTP host/port/user/pass for email) as opaque JSON so the
// schema never has to change when a new vendor is added.
type NotifyChannel struct {
	ID        string
	Kind      NotifyChannelKind
	Provider  string
	Config    map[string]any
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
