// Package notify sends a message through whichever SMS or email vendor
// an admin has configured (DESIGN.md 7.1: "短信和邮件都做成可插拔的发信
// 通道...不写死某一家服务商"). It has no database access of its own --
// the caller (the user module, for local-account OTP codes) loads the
// channel's config and passes it in here.
package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/amigoer/fluxa/internal/notify/mail"
	"github.com/amigoer/fluxa/internal/notify/sms"
)

// Sender delivers message to recipient using whatever credentials are in
// config. Each vendor (Aliyun SMS, plain SMTP, ...) implements this the
// same way, so adding one is a new file, not a change to callers.
type Sender interface {
	Send(ctx context.Context, config map[string]any, recipient, message string) error
}

// EmailSender is the same contract plus a subject, which SMS has no
// concept of. The two are separate interfaces so a channel test and a
// verification code can arrive under different subject lines -- with one
// shared signature every mail this system sends had to claim to be a
// verification code.
type EmailSender interface {
	SendEmail(ctx context.Context, config map[string]any, recipient, subject, body string) error
}

// requiredConfig lists the fields each vendor cannot send without. It
// lives here for the same reason the sender maps do: adding a vendor
// should be one file, not an edit spread across the user module.
var requiredConfig = map[string][]string{
	"aliyun_sms": {"access_key_id", "access_key_secret", "sign_name", "template_code"},
	"smtp":       {"host", "port", "from_address"},
}

// Configured reports whether config carries everything provider needs to
// actually deliver.
//
// An enabled-but-empty channel is worse than a disabled one: it satisfies
// every "is a channel switched on?" check while failing at the moment
// somebody depends on it, which is how the login page came to advertise a
// method that could not send.
func Configured(provider string, config map[string]any) bool {
	required, ok := requiredConfig[provider]
	if !ok {
		return false
	}
	for _, key := range required {
		if v, _ := config[key].(string); strings.TrimSpace(v) == "" {
			return false
		}
	}
	return true
}

var smsSenders = map[string]Sender{
	"aliyun_sms": sms.NewAliyunSender(),
}

var emailSenders = map[string]EmailSender{
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

// SendEmail delivers body to an email address through the named email
// vendor, under subject.
func SendEmail(ctx context.Context, provider string, config map[string]any, recipient, subject, body string) error {
	sender, ok := emailSenders[provider]
	if !ok {
		return fmt.Errorf("notify: unknown email provider %q", provider)
	}
	return sender.SendEmail(ctx, config, recipient, subject, body)
}
