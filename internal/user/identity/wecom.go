package identity

import (
	"context"
	"errors"
)

// WeComAdapter is a reserved slot for WeCom (企业微信) login (DESIGN.md
// 5: "后续可扩展企业微信/钉钉"). Not implemented in v1; wiring it in is
// adding one adapter, not touching the login flow.
type WeComAdapter struct{}

func NewWeComAdapter() *WeComAdapter { return &WeComAdapter{} }

func (a *WeComAdapter) ExchangeCode(ctx context.Context, appID, appSecret, code string) (UserInfo, error) {
	return UserInfo{}, errors.New("identity: wecom adapter not implemented yet")
}
