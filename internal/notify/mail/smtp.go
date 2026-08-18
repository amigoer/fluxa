// Package mail holds one Sender implementation for plain SMTP.
package mail

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// dialTimeout bounds the TCP connect and the TLS handshake. net/smtp has
// no timeout of its own, so without this a black-holed host hangs the
// request until the server's own 60s timeout fires.
const dialTimeout = 15 * time.Second

// conversationTimeout bounds everything after the connect: the greeting,
// every command, and the message body. Connecting is not the only place a
// relay can stall -- one that accepts the connection and then says
// nothing would otherwise hang the request forever, since net/smtp reads
// the greeting with no deadline and takes no context. A var so tests can
// shrink it.
var conversationTimeout = 30 * time.Second

// applyDeadline puts a hard stop on the whole exchange, honouring an
// earlier deadline from the caller's context if it has one.
func applyDeadline(ctx context.Context, conn net.Conn) {
	deadline := time.Now().Add(conversationTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
}

// tlsRootCAs overrides the roots used to verify a relay's certificate.
// Production leaves it nil, which means the system pool; the tests set it
// to trust the throwaway CA they sign their test relay with, so the TLS
// paths run for real instead of being asserted about.
var tlsRootCAs *x509.CertPool

func tlsConfig(host string) *tls.Config {
	return &tls.Config{ServerName: host, RootCAs: tlsRootCAs}
}

// SMTPSender sends email through a configured SMTP relay. config is
// expected to hold: host, port, username, password, from_address, and
// optionally from_name.
type SMTPSender struct{}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

// SendEmail delivers one message. Two things here are what make it work
// against real providers rather than only a local relay:
//
//   - Port 465 is implicit TLS: the connection is wrapped before any SMTP
//     is spoken. Everything else is dialled plain and upgraded with
//     STARTTLS when the server offers it. net/smtp's SendMail only does
//     the second, which is why 465 (what 163/QQ hand out first) failed.
//   - The body is a real MIME message declaring UTF-8, and the subject and
//     sender name go through RFC 2047. Without that a Chinese subject
//     arrives as mojibake, and plenty of relays score the message as spam.
func (s *SMTPSender) SendEmail(ctx context.Context, config map[string]any, recipient string, mail Mail) error {
	host, _ := config["host"].(string)
	port, _ := config["port"].(string)
	username, _ := config["username"].(string)
	password, _ := config["password"].(string)
	fromAddress, _ := config["from_address"].(string)
	fromName, _ := config["from_name"].(string)
	if host == "" || port == "" || fromAddress == "" {
		return fmt.Errorf("mail: smtp channel is missing host, port or from_address")
	}

	client, err := dial(ctx, host, port)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig(host)); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}

	if username != "" {
		// LOGIN is offered by several China-based relays that do not
		// advertise PLAIN; net/smtp only implements PLAIN and CRAM-MD5.
		auth := smtp.Auth(smtp.PlainAuth("", username, password, host))
		if ok, mechs := client.Extension("AUTH"); ok && !strings.Contains(mechs, "PLAIN") &&
			strings.Contains(mechs, "LOGIN") {
			auth = &loginAuth{username: username, password: password}
		}
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}

	if err := client.Mail(fromAddress); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("mail: to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write([]byte(message(fromAddress, fromName, recipient, mail))); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return client.Quit()
}

// implicitTLSPorts are the ports on which the connection is encrypted
// from the first byte, with no plaintext greeting to upgrade. 465 is the
// only one in real use; it is a map rather than a literal so the tests
// can add the ephemeral port their TLS relay landed on.
var implicitTLSPorts = map[string]bool{"465": true}

// dial opens the connection, wrapping it in TLS up front on 465.
func dial(ctx context.Context, host, port string) (*smtp.Client, error) {
	addr := net.JoinHostPort(host, port)
	d := &net.Dialer{Timeout: dialTimeout}

	if implicitTLSPorts[port] {
		conn, err := tls.DialWithDialer(d, "tcp", addr, tlsConfig(host))
		if err != nil {
			return nil, fmt.Errorf("mail: dial %s over tls: %w", addr, err)
		}
		applyDeadline(ctx, conn)
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("mail: smtp handshake: %w", err)
		}
		return client, nil
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	applyDeadline(ctx, conn)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mail: smtp handshake: %w", err)
	}
	return client, nil
}

// message builds the MIME document. With an HTML part it is a
// multipart/alternative carrying both, text first: that order is the
// spec's -- the last part is the client's preferred one -- and a client
// that understands neither still shows something readable.
func message(fromAddress, fromName, recipient string, mail Mail) string {
	from := fromAddress
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", fromName), fromAddress)
	}

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + recipient + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", mail.Subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if mail.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		// Written raw: the writer from Client.Data is a textproto.DotWriter,
		// which already escapes a leading "." and terminates the block. Doing
		// it here too would put the second dot on the wire for real.
		b.WriteString(mail.Text)
		b.WriteString("\r\n")
		return b.String()
	}

	boundary := boundaryFor(mail)
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")
	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=UTF-8", mail.Text},
		{"text/html; charset=UTF-8", mail.HTML},
	} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + part.ctype + "\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(part.body)
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

// boundaryFor derives a delimiter that cannot occur inside either part,
// which is the one thing a boundary must guarantee. Random would do as
// well, but deriving it keeps the message byte-identical for the same
// input and so keeps the tests meaningful.
func boundaryFor(mail Mail) string {
	sum := sha256.Sum256([]byte(mail.Text + mail.HTML))
	return "fluxa-" + hex.EncodeToString(sum[:8])
}

// loginAuth implements the non-standard but widely deployed AUTH LOGIN.
type loginAuth struct{ username, password string }

// Start refuses to hand the password to an unencrypted connection, which
// is the same guard smtp.PlainAuth applies. Without it this type would be
// a quiet downgrade: choosing LOGIN over PLAIN would also be choosing to
// put the credential on the wire in the clear.
//
// A loopback relay is exempted for the same reason net/smtp exempts it --
// there is no network to intercept, and local test relays speak plaintext.
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errors.New("mail: refusing to send credentials over an unencrypted connection")
	}
	return "LOGIN", nil, nil
}

func isLocalhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimRight(string(fromServer), ": ")) {
	case "username":
		return []byte(a.username), nil
	case "password":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("mail: unexpected login prompt %q", fromServer)
	}
}
