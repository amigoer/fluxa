// Package mail holds one Sender implementation for plain SMTP.
package mail

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPSender sends email through a configured SMTP relay. config is
// expected to hold: host, port, username, password, from_address, and
// optionally from_name.
type SMTPSender struct{}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

func (s *SMTPSender) Send(ctx context.Context, config map[string]any, recipient, message string) error {
	host, _ := config["host"].(string)
	port, _ := config["port"].(string)
	username, _ := config["username"].(string)
	password, _ := config["password"].(string)
	fromAddress, _ := config["from_address"].(string)
	fromName, _ := config["from_name"].(string)
	if host == "" || port == "" || fromAddress == "" {
		return fmt.Errorf("mail: smtp channel is missing required config")
	}

	from := fromAddress
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromAddress)
	}

	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Fluxa verification code\r\n\r\n%s\r\n",
		from, recipient, message)

	addr := host + ":" + port
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	// net/smtp doesn't take a context; the caller applies its own
	// timeout to the surrounding request instead.
	_ = ctx
	return smtp.SendMail(addr, auth, fromAddress, []string{recipient}, []byte(body))
}
