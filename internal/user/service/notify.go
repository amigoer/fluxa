package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// NotifyService owns the outbound channel configuration verification
// codes are delivered through. Sending is the handler's job, against the
// notify package; this is only where a channel is read and saved.
type NotifyService interface {
	GetNotifyChannel(ctx context.Context, kind types.NotifyChannelKind) (types.NotifyChannel, error)
	UpsertNotifyChannel(ctx context.Context, c types.NotifyChannel) error
}

func (s *service) GetNotifyChannel(ctx context.Context, kind types.NotifyChannelKind) (types.NotifyChannel, error) {
	return s.repo.GetNotifyChannel(ctx, kind)
}

func (s *service) UpsertNotifyChannel(ctx context.Context, c types.NotifyChannel) error {
	return s.repo.UpsertNotifyChannel(ctx, c)
}
