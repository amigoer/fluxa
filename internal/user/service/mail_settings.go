package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/notify"
	"github.com/amigoer/fluxa/internal/user/types"
)

// MailSettingsService owns the wording on outbound mail, and is also
// where it is turned into what the templates take. Callers ask for the
// brand rather than the row: every one of them wants it to send with,
// and none of them should have to know which fields map to which.
type MailSettingsService interface {
	GetMailSettings(ctx context.Context) (types.MailSettings, error)
	UpdateMailSettings(ctx context.Context, s types.MailSettings) error
	MailBrand(ctx context.Context) notify.Brand
}

func (s *service) GetMailSettings(ctx context.Context) (types.MailSettings, error) {
	return s.repo.GetMailSettings(ctx)
}

func (s *service) UpdateMailSettings(ctx context.Context, settings types.MailSettings) error {
	return s.repo.UpdateMailSettings(ctx, settings)
}

// MailBrand swallows a read failure and returns the zero value, whose
// fields all fall back to the built-in wording. A verification code that
// somebody is waiting on must not fail to send because the row that only
// decides the footer could not be read.
func (s *service) MailBrand(ctx context.Context) notify.Brand {
	settings, err := s.repo.GetMailSettings(ctx)
	if err != nil {
		return notify.Brand{}
	}
	return notify.Brand{
		OrgName: settings.OrgName,
		SignOff: settings.SignOff,
		Contact: settings.ContactLine,
	}
}
