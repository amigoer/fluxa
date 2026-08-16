package mail

import (
	"strings"
	"testing"
)

func TestMessageEncodesNonASCIIHeaders(t *testing.T) {
	got := message("fluxa@example.com", "Fluxa 网关", "user@example.com", "Fluxa 验证码", "验证码是 123456")

	// Raw UTF-8 in a header is what made subjects arrive as mojibake and
	// pushed the message toward spam folders, so both headers have to come
	// out RFC 2047 encoded rather than as the literal runes.
	for _, want := range []string{
		"From: =?UTF-8?q?Fluxa_=E7=BD=91=E5=85=B3?= <fluxa@example.com>",
		"Subject: =?UTF-8?q?Fluxa_=E9=AA=8C=E8=AF=81=E7=A0=81?=",
		"To: user@example.com",
		"Content-Type: text/plain; charset=UTF-8",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message() missing %q\ngot:\n%s", want, got)
		}
	}

	// The body stays plain UTF-8: it is the headers that need encoding.
	if !strings.Contains(got, "\r\n\r\n验证码是 123456\r\n") {
		t.Errorf("message() body not written verbatim after the header break\ngot:\n%s", got)
	}
}

func TestMessageOmitsFromNameWhenUnset(t *testing.T) {
	got := message("fluxa@example.com", "", "user@example.com", "hi", "body")
	if !strings.Contains(got, "From: fluxa@example.com\r\n") {
		t.Errorf("message() should use the bare address when no name is set\ngot:\n%s", got)
	}
}

func TestMessageLeavesDotStuffingToTheWriter(t *testing.T) {
	// smtp.Client.Data hands back a textproto.DotWriter, which escapes a
	// leading "." itself. Doubling it here as well would send three dots
	// for two and corrupt the body.
	got := message("fluxa@example.com", "", "user@example.com", "hi", "line one\r\n.hidden\r\nline three")
	if strings.Contains(got, "\r\n..hidden") {
		t.Errorf("message() double-stuffed a leading dot\ngot:\n%s", got)
	}
	if !strings.Contains(got, "\r\n.hidden\r\n") {
		t.Errorf("message() altered a body line starting with a dot\ngot:\n%s", got)
	}
}
