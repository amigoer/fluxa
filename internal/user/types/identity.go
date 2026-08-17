package types

import "time"

type IdentityProvider string

const (
	IdentityProviderFeishu   IdentityProvider = "feishu"
	IdentityProviderWeCom    IdentityProvider = "wecom"
	IdentityProviderDingTalk IdentityProvider = "dingtalk"
)

type ExternalIdentity struct {
	ID             string
	MemberID       string
	Provider       IdentityProvider
	ExternalUserID string
	CreatedAt      time.Time
}

// IdentityConfig holds the OAuth app credentials an admin configures for
// one identity provider, instead of baking them into config files
// (DESIGN.md 7.1).
type IdentityConfig struct {
	ID           string
	Provider     IdentityProvider
	AppID        string
	AppSecret    string
	CallbackPath string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
