package repo

import (
	"context"

	"github.com/amigoer/fluxa/internal/user/types"
)

// MailSettingsRepo stores the one row of mail wording. Shaped like
// AuthSettingsRepo because it is the same kind of thing: a handful of
// deployment-wide values with exactly one row behind them.
type MailSettingsRepo interface {
	GetMailSettings(ctx context.Context) (types.MailSettings, error)
	UpdateMailSettings(ctx context.Context, s types.MailSettings) error
}

func (r *repo) GetMailSettings(ctx context.Context) (types.MailSettings, error) {
	var s types.MailSettings
	err := r.pool.QueryRow(ctx,
		`SELECT org_name, sign_off, contact_line FROM mail_settings`,
	).Scan(&s.OrgName, &s.SignOff, &s.ContactLine)
	return s, err
}

func (r *repo) UpdateMailSettings(ctx context.Context, s types.MailSettings) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE mail_settings SET org_name = $1, sign_off = $2, contact_line = $3, updated_at = now()`,
		s.OrgName, s.SignOff, s.ContactLine,
	)
	return err
}
