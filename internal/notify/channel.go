// Package notify sends a message through whichever SMS or email vendor
// an admin has configured (DESIGN.md 7.1: "短信和邮件都做成可插拔的发信
// 通道...不写死某一家服务商"). It has no database access of its own --
// the caller (the user module, for local-account OTP codes) loads the
// channel's config and passes it in here.
package notify

import (
	"context"
	"fmt"

	"github.com/amigoer/fluxa/internal/notify/mail"
	"github.com/amigoer/fluxa/internal/notify/sms"
)

// Sender delivers message to recipient using whatever credentials are in
// config. Each vendor (Aliyun SMS, plain SMTP, ...) implements this the
// same way, so adding one is a new file, not a change to callers.
type Sender interface {
	Send(ctx context.Context, config map[string]any, recipient, message string) error
}

var smsSenders = map[string]Sender{
	"aliyun_sms": sms.NewAliyunSender(),
}

var emailSenders = map[string]Sender{
	"smtp": mail.NewSMTPSender(),
}

// SendSMS delivers message to a phone number through the named SMS
// vendor.
func SendSMS(ctx context.Context, provider string, config map[string]any, recipient, message string) error {
	sender, ok := smsSenders[provider]
	if !ok {
		return fmt.Errorf("notify: unknown sms provider %q", provider)
	}
	return sender.Send(ctx, config, recipient, message)
}

// SendEmail delivers message to an email address through the named
// email vendor.
func SendEmail(ctx context.Context, provider string, config map[string]any, recipient, message string) error {
	sender, ok := emailSenders[provider]
	if !ok {
		return fmt.Errorf("notify: unknown email provider %q", provider)
	}
	return sender.Send(ctx, config, recipient, message)
}
