package gateway

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The signing key derivation has a published test vector, which is the
// part of SigV4 worth pinning: everything else is string assembly, but a
// wrong key produces a signature that fails only against the real AWS.
//
// https://docs.aws.amazon.com/general/latest/gr/signature-v4-examples.html
func TestDeriveSigningKeyMatchesTheAWSTestVector(t *testing.T) {
	got := deriveSigningKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "20150830", "us-east-1", "iam")
	const want = "c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9"
	if hex.EncodeToString(got) != want {
		t.Errorf("signing key = %s\nwant           %s", hex.EncodeToString(got), want)
	}
}

// Go sends the host from the URL and never from req.Header, so a
// signature over anything else is a signature over a request that was
// not sent.
func TestCanonicalHeadersSignTheHostThatIsActuallySent(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/model/x/converse", nil)
	req.Host = "" // as an outbound client request has it

	canonical, signed := canonicalHeaderList(req)
	if !strings.Contains(canonical, "host:bedrock-runtime.us-east-1.amazonaws.com\n") {
		t.Errorf("canonical headers = %q", canonical)
	}
	if !strings.Contains(signed, "host") {
		t.Errorf("signed headers = %q, want host among them", signed)
	}
}

// The old signer emitted query values raw. Anything needing an escape
// then signed a different string than the one on the wire.
func TestCanonicalQueryEscapesAndSorts(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://x/?b=2&a=hello+world&a=%2Ffoo", nil)
	got := canonicalQuery(req)
	if got != "a=%2Ffoo&a=hello+world&b=2" {
		t.Errorf("canonical query = %q", got)
	}
}

func TestSignV4SetsTheHeadersAWSRequires(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/model/x/converse", nil)
	body := []byte(`{"messages":[]}`)

	signV4(req, body, "AKIDEXAMPLE", "secret", "session-token", "us-east-1", bedrockService,
		time.Date(2026, 8, 20, 12, 36, 0, 0, time.UTC))

	if got := req.Header.Get("X-Amz-Date"); got != "20260820T123600Z" {
		t.Errorf("X-Amz-Date = %q", got)
	}
	if req.Header.Get("X-Amz-Security-Token") != "session-token" {
		t.Error("session token was not sent")
	}
	if got := req.Header.Get("X-Amz-Content-Sha256"); got != hashHex(body) {
		t.Errorf("payload hash = %q", got)
	}
	auth := req.Header.Get("Authorization")
	for _, want := range []string{
		"AWS4-HMAC-SHA256",
		"Credential=AKIDEXAMPLE/20260820/us-east-1/bedrock/aws4_request",
		"SignedHeaders=",
		"Signature=",
	} {
		if !strings.Contains(auth, want) {
			t.Errorf("Authorization = %q, missing %q", auth, want)
		}
	}
	// Everything signed must actually be on the request, or AWS rejects it.
	for _, name := range strings.Split(signedHeadersOf(auth), ";") {
		if name == "host" {
			continue
		}
		if req.Header.Get(name) == "" {
			t.Errorf("signed header %q is not present on the request", name)
		}
	}
}

func signedHeadersOf(auth string) string {
	for _, part := range strings.Split(auth, ", ") {
		if strings.HasPrefix(part, "SignedHeaders=") {
			return strings.TrimPrefix(part, "SignedHeaders=")
		}
	}
	return ""
}

// A signature is only useful if it is stable for the same inputs.
func TestSignV4IsDeterministic(t *testing.T) {
	at := time.Date(2026, 8, 20, 12, 36, 0, 0, time.UTC)
	sign := func() string {
		req := httptest.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/model/x/converse", nil)
		signV4(req, []byte(`{"a":1}`), "AKID", "secret", "", "us-east-1", bedrockService, at)
		return req.Header.Get("Authorization")
	}
	if sign() != sign() {
		t.Error("signing the same request twice produced different signatures")
	}
}
