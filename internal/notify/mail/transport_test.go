package mail

import (
	"context"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

// send drives the real sender against a fake relay.
func send(t *testing.T, r *relay) error {
	t.Helper()
	return NewSMTPSender().SendEmail(context.Background(), r.config(), "user@example.com", "Fluxa 测试邮件", "正文")
}

func TestSendEmailOverImplicitTLS(t *testing.T) {
	// 465 speaks TLS from the first byte. net/smtp's SendMail cannot do
	// this at all, which is what made 163/QQ -- who hand out 465 first --
	// fail before the rewrite.
	r := newRelay(t, relayOpts{implicitTLS: true, authMechs: "PLAIN LOGIN"})

	implicitTLSPorts[r.port] = true
	t.Cleanup(func() { delete(implicitTLSPorts, r.port) })

	if err := send(t, r); err != nil {
		t.Fatalf("SendEmail over implicit TLS: %v", err)
	}
	got := r.snapshot()
	if !got.authTLS {
		t.Error("credentials were sent over an unencrypted session")
	}
	if !strings.Contains(got.body, "正文") {
		t.Errorf("body not delivered, got %q", got.body)
	}
}

func TestSendEmailUpgradesWithStartTLS(t *testing.T) {
	r := newRelay(t, relayOpts{startTLS: true, authMechs: "PLAIN LOGIN"})

	if err := send(t, r); err != nil {
		t.Fatalf("SendEmail with STARTTLS: %v", err)
	}
	got := r.snapshot()
	if !got.authTLS {
		t.Error("credentials were sent before the STARTTLS upgrade")
	}
	if !strings.Contains(got.body, "正文") {
		t.Errorf("body not delivered, got %q", got.body)
	}
}

func TestSendEmailFallsBackToLoginAuth(t *testing.T) {
	// The relay offers only LOGIN. net/smtp implements PLAIN and CRAM-MD5
	// and would fail here, which is the case several China-based relays
	// present.
	r := newRelay(t, relayOpts{startTLS: true, authMechs: "LOGIN"})

	if err := send(t, r); err != nil {
		t.Fatalf("SendEmail against a LOGIN-only relay: %v", err)
	}
	got := r.snapshot()
	if got.authMech != "LOGIN" {
		t.Errorf("chose %q, want LOGIN", got.authMech)
	}
	if got.authUser != "sender@relay.test" || got.authPass != "s3cr3t" {
		t.Errorf("credentials arrived as %q/%q", got.authUser, got.authPass)
	}
}

func TestSendEmailPrefersPlainWhenOffered(t *testing.T) {
	// PLAIN is the standard mechanism; LOGIN is the fallback, not the
	// default. Getting this backwards would work but would mean always
	// speaking the non-standard one.
	r := newRelay(t, relayOpts{startTLS: true, authMechs: "PLAIN LOGIN"})

	if err := send(t, r); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if got := r.snapshot(); got.authMech != "PLAIN" {
		t.Errorf("chose %q, want PLAIN", got.authMech)
	}
}

func TestLoginAuthRefusesCleartextCredentials(t *testing.T) {
	a := &loginAuth{username: "u", password: "p"}

	if _, _, err := a.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false}); err == nil {
		t.Error("Start() sent credentials over an unencrypted connection to a remote relay")
	}
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true}); err != nil {
		t.Errorf("Start() over TLS = %v, want nil", err)
	}
	// A loopback relay has no network to intercept, and local test relays
	// speak plaintext -- net/smtp makes the same exception.
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "127.0.0.1", TLS: false}); err != nil {
		t.Errorf("Start() against loopback = %v, want nil", err)
	}
}

func TestSendEmailGivesUpOnASilentRelay(t *testing.T) {
	// Accepts the connection, then never speaks. The dial timeout does not
	// help here -- the connect already succeeded -- so without a deadline
	// on the conversation this hangs until the caller gives up.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	prev := conversationTimeout
	conversationTimeout = 200 * time.Millisecond
	t.Cleanup(func() { conversationTimeout = prev })

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	config := map[string]any{"host": host, "port": port, "from_address": "sender@relay.test"}

	done := make(chan error, 1)
	go func() {
		done <- NewSMTPSender().SendEmail(context.Background(), config, "user@example.com", "s", "b")
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("SendEmail() = nil, want a timeout error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendEmail() hung on a relay that accepted the connection and went silent")
	}
}

func TestPort465IsImplicitTLSByDefault(t *testing.T) {
	if !implicitTLSPorts["465"] {
		t.Error("465 must be treated as implicit TLS")
	}
	if implicitTLSPorts["587"] || implicitTLSPorts["25"] {
		t.Error("587 and 25 start in plaintext and upgrade with STARTTLS")
	}
}
