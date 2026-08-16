package mail

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// A fake SMTP relay, because the paths worth testing here are exactly the
// ones a plaintext catcher cannot reach: implicit TLS on connect, the
// STARTTLS upgrade, and the AUTH mechanism negotiation. Each of those is
// a decision made against what the server says, so the server has to be
// able to say the awkward things a real provider says.

type relayOpts struct {
	implicitTLS bool   // serve TLS from the first byte, as 465 does
	startTLS    bool   // advertise STARTTLS and honour it
	authMechs   string // advertised after "AUTH ", empty to advertise none
}

type relay struct {
	host, port string

	mu       sync.Mutex
	authMech string // mechanism the client actually chose
	authUser string
	authPass string
	authTLS  bool // was the session encrypted when credentials were sent
	body     string
	rcpt     string
}

// newRelay starts a relay on a loopback ephemeral port and stops it when
// the test ends.
func newRelay(t *testing.T, opts relayOpts) *relay {
	t.Helper()

	cert, pool := newTestCert(t)
	tlsConf := &tls.Config{Certificates: []tls.Certificate{cert}}

	var ln net.Listener
	var err error
	if opts.implicitTLS {
		ln, err = tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	} else {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	// Point the sender's verification at the throwaway CA for the duration
	// of the test, so the handshake is real rather than skipped.
	prev := tlsRootCAs
	tlsRootCAs = pool
	t.Cleanup(func() { tlsRootCAs = prev })

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	r := &relay{host: host, port: port}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go r.serve(conn, tlsConf, opts)
		}
	}()
	return r
}

func (r *relay) serve(conn net.Conn, tlsConf *tls.Config, opts relayOpts) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	isTLS := opts.implicitTLS
	br := bufio.NewReader(conn)
	write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }

	ehlo := func() {
		write("250-relay.test")
		if opts.startTLS && !isTLS {
			write("250-STARTTLS")
		}
		if opts.authMechs != "" {
			write("250-AUTH " + opts.authMechs)
		}
		write("250 SIZE 10485760")
	}

	write("220 relay.test ESMTP")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		upper := strings.ToUpper(cmd)

		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			ehlo()

		case upper == "STARTTLS":
			write("220 ready")
			tlsConn := tls.Server(conn, tlsConf)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			br = bufio.NewReader(conn)
			write = func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
			isTLS = true

		case strings.HasPrefix(upper, "AUTH PLAIN"):
			r.record(func() {
				r.authMech, r.authTLS = "PLAIN", isTLS
				// PLAIN is NUL-separated: identity\0username\0password.
				if fields := strings.Fields(cmd); len(fields) == 3 {
					if raw, err := base64.StdEncoding.DecodeString(fields[2]); err == nil {
						if parts := strings.Split(string(raw), "\x00"); len(parts) == 3 {
							r.authUser, r.authPass = parts[1], parts[2]
						}
					}
				}
			})
			write("235 2.7.0 accepted")

		case strings.HasPrefix(upper, "AUTH LOGIN"):
			r.record(func() { r.authMech, r.authTLS = "LOGIN", isTLS })
			write("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			user, _ := br.ReadString('\n')
			write("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			pass, _ := br.ReadString('\n')
			r.record(func() {
				r.authUser = decode64(user)
				r.authPass = decode64(pass)
			})
			write("235 2.7.0 accepted")

		case strings.HasPrefix(upper, "MAIL FROM"):
			write("250 2.1.0 ok")

		case strings.HasPrefix(upper, "RCPT TO"):
			r.record(func() { r.rcpt = cmd })
			write("250 2.1.0 ok")

		case upper == "DATA":
			write("354 end with <CR><LF>.<CR><LF>")
			var b strings.Builder
			for {
				dl, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if dl == ".\r\n" || dl == ".\n" {
					break
				}
				// Undo the transport's dot-stuffing so assertions see the
				// body the caller actually passed in.
				b.WriteString(strings.TrimPrefix(dl, "."))
			}
			r.record(func() { r.body = b.String() })
			write("250 2.0.0 queued")

		case upper == "QUIT":
			write("221 2.0.0 bye")
			return

		default:
			write("250 2.0.0 ok")
		}
	}
}

func (r *relay) record(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f()
}

func (r *relay) snapshot() relay {
	r.mu.Lock()
	defer r.mu.Unlock()
	return relay{authMech: r.authMech, authUser: r.authUser, authPass: r.authPass, authTLS: r.authTLS, body: r.body, rcpt: r.rcpt}
}

// config is the channel config an admin would have saved for this relay.
func (r *relay) config() map[string]any {
	return map[string]any{
		"host":         r.host,
		"port":         r.port,
		"username":     "sender@relay.test",
		"password":     "s3cr3t",
		"from_address": "sender@relay.test",
		"from_name":    "Fluxa 网关",
	}
}

func decode64(line string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(line))
	if err != nil {
		return ""
	}
	return string(raw)
}

// newTestCert returns a self-signed certificate valid for 127.0.0.1 and
// the pool that trusts it.
func newTestCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "relay.test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost", "relay.test"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
