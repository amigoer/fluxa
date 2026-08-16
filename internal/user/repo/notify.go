package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/amigoer/fluxa/internal/user/types"
)

// NotifyRepo stores the outbound channel configuration verification
// codes go out through, plus the delivery log the console reads usage
// and failures from.
type NotifyRepo interface {
	GetNotifyChannel(ctx context.Context, kind types.NotifyChannelKind) (types.NotifyChannel, error)
	UpsertNotifyChannel(ctx context.Context, c types.NotifyChannel) error
	LogNotifySent(ctx context.Context, kind types.NotifyChannelKind, recipient, purpose string) error
	LogNotifyFailed(ctx context.Context, kind types.NotifyChannelKind, recipient, purpose string, cause error) error
	CountNotifySentThisMonth(ctx context.Context, kind types.NotifyChannelKind) (int, error)
}

// Outcomes recorded in notify_log.
const (
	NotifyStatusSent   = "sent"
	NotifyStatusFailed = "failed"
)

// failureTextLimit keeps one pathological upstream message from bloating
// the row. Every SMTP error worth reading fits well inside it.
const failureTextLimit = 500

func (r *repo) GetNotifyChannel(ctx context.Context, kind types.NotifyChannelKind) (types.NotifyChannel, error) {
	var c types.NotifyChannel
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, kind, provider, config, enabled, created_at, updated_at
		FROM notify_channels WHERE kind = $1`, kind,
	).Scan(&c.ID, &c.Kind, &c.Provider, &raw, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.NotifyChannel{}, ErrNotFound
	}
	if err != nil {
		return types.NotifyChannel{}, err
	}
	if err := json.Unmarshal(raw, &c.Config); err != nil {
		return types.NotifyChannel{}, fmt.Errorf("user: decode notify channel config: %w", err)
	}
	return c, nil
}

func (r *repo) UpsertNotifyChannel(ctx context.Context, c types.NotifyChannel) error {
	raw, err := json.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("user: encode notify channel config: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO notify_channels (kind, provider, config, enabled, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (kind) DO UPDATE SET
			provider = EXCLUDED.provider,
			config = EXCLUDED.config,
			enabled = EXCLUDED.enabled,
			updated_at = now()`,
		c.Kind, c.Provider, raw, c.Enabled,
	)
	return err
}

// LogNotifySent records a delivered message.
func (r *repo) LogNotifySent(ctx context.Context, kind types.NotifyChannelKind, recipient, purpose string) error {
	return r.logNotify(ctx, kind, recipient, purpose, NotifyStatusSent, "")
}

// LogNotifyFailed records an attempt the vendor rejected, with the reason.
// Failures were previously not recorded at all, which left "the code never
// arrived" with nothing on the server side to look at.
func (r *repo) LogNotifyFailed(ctx context.Context, kind types.NotifyChannelKind, recipient, purpose string, cause error) error {
	text := cause.Error()
	if len(text) > failureTextLimit {
		text = text[:failureTextLimit]
	}
	return r.logNotify(ctx, kind, recipient, purpose, NotifyStatusFailed, text)
}

func (r *repo) logNotify(ctx context.Context, kind types.NotifyChannelKind, recipient, purpose, status, failure string) error {
	var errText *string
	if failure != "" {
		errText = &failure
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notify_log (kind, recipient, purpose, status, error) VALUES ($1, $2, $3, $4, $5)`,
		kind, recipient, purpose, status, errText,
	)
	return err
}

// CountNotifySentThisMonth counts delivered messages only: a failed
// attempt costs no quota at the vendor and should not read as usage.
func (r *repo) CountNotifySentThisMonth(ctx context.Context, kind types.NotifyChannelKind) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM notify_log
		WHERE kind = $1 AND status = $2 AND sent_at >= date_trunc('month', now())`,
		kind, NotifyStatusSent,
	).Scan(&n)
	return n, err
}
