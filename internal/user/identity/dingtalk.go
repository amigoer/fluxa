package identity

import (
	"context"
	"errors"
)

// DingTalkAdapter is a reserved slot for DingTalk (钉钉) login, same
// status as WeComAdapter: not implemented in v1.
type DingTalkAdapter struct{}

func NewDingTalkAdapter() *DingTalkAdapter { return &DingTalkAdapter{} }

func (a *DingTalkAdapter) ExchangeCode(ctx context.Context, appID, appSecret, code string) (UserInfo, error) {
	return UserInfo{}, errors.New("identity: dingtalk adapter not implemented yet")
}
