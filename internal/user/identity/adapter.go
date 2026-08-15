// Package identity defines the pluggable adapter interface every IM
// login provider implements (DESIGN.md 7.1: "身份源做成可插拔的适配器
// 设计"), so adding WeCom or DingTalk later is a new file, not a rewrite
// of the login flow.
package identity

import "context"

// UserInfo is what an adapter returns after a successful OAuth exchange:
// just enough to find or create the matching member.
type UserInfo struct {
	ExternalUserID string
	Name           string
	Email          string
}

// Adapter exchanges an OAuth authorization code for the caller's
// identity at the provider.
type Adapter interface {
	// ExchangeCode trades an OAuth authorization code for the user's
	// identity, using the given app credentials.
	ExchangeCode(ctx context.Context, appID, appSecret, code string) (UserInfo, error)
}
