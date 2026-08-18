package mail

import (
	"strings"
	"testing"
)

func TestMessageEncodesNonASCIIHeaders(t *testing.T) {
	got := message("fluxa@example.com", "Fluxa 网关", "user@example.com",
		Mail{Subject: "Fluxa 验证码", Text: "验证码是 123456"})

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
	got := message("fluxa@example.com", "", "user@example.com", Mail{Subject: "hi", Text: "body"})
	if !strings.Contains(got, "From: fluxa@example.com\r\n") {
		t.Errorf("message() should use the bare address when no name is set\ngot:\n%s", got)
	}
}

func TestMessageLeavesDotStuffingToTheWriter(t *testing.T) {
	// smtp.Client.Data hands back a textproto.DotWriter, which escapes a
	// leading "." itself. Doubling it here as well would send three dots
	// for two and corrupt the body.
	got := message("fluxa@example.com", "", "user@example.com",
		Mail{Subject: "hi", Text: "line one\r\n.hidden\r\nline three"})
	if strings.Contains(got, "\r\n..hidden") {
		t.Errorf("message() double-stuffed a leading dot\ngot:\n%s", got)
	}
	if !strings.Contains(got, "\r\n.hidden\r\n") {
		t.Errorf("message() altered a body line starting with a dot\ngot:\n%s", got)
	}
}

func TestMessageCarriesBothPartsWhenHTMLIsSet(t *testing.T) {
	m := Mail{Subject: "hi", Text: "plain body", HTML: "<p>rich body</p>"}
	got := message("fluxa@example.com", "", "user@example.com", m)

	if !strings.Contains(got, "Content-Type: multipart/alternative; boundary=\"") {
		t.Fatalf("message() should be multipart when HTML is set\ngot:\n%s", got)
	}

	// Text first, HTML last: the spec has the client prefer the final
	// part, so reversing these would show the markup to nobody.
	text, html := strings.Index(got, "plain body"), strings.Index(got, "<p>rich body</p>")
	if text < 0 || html < 0 || text > html {
		t.Errorf("message() should carry text before HTML\ngot:\n%s", got)
	}
	for _, want := range []string{"Content-Type: text/plain; charset=UTF-8", "Content-Type: text/html; charset=UTF-8"} {
		if !strings.Contains(got, want) {
			t.Errorf("message() missing part header %q\ngot:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "--\r\n") {
		t.Errorf("message() should end with the closing boundary\ngot:\n%s", got)
	}
}

func TestMessageBoundaryCannotCollideWithTheBody(t *testing.T) {
	// A boundary that occurs inside a part truncates the message at that
	// point, so it is derived from the content rather than picked.
	m := Mail{Subject: "hi", Text: "plain", HTML: "<p>rich</p>"}
	got := message("fluxa@example.com", "", "user@example.com", m)

	line := ""
	for _, l := range strings.Split(got, "\r\n") {
		if strings.HasPrefix(l, "Content-Type: multipart/alternative") {
			line = l
			break
		}
	}
	boundary := strings.Trim(strings.SplitN(line, "boundary=", 2)[1], "\"")
	if boundary == "" {
		t.Fatal("no boundary in the multipart header")
	}
	if strings.Contains(m.Text, boundary) || strings.Contains(m.HTML, boundary) {
		t.Errorf("boundary %q occurs inside a part", boundary)
	}
}

func TestMessageStaysSinglePartWithoutHTML(t *testing.T) {
	// The plain path is what every existing caller used; it must not grow
	// multipart framing a text-only client would then show as raw text.
	got := message("fluxa@example.com", "", "user@example.com", Mail{Subject: "hi", Text: "body"})
	if strings.Contains(got, "multipart/alternative") {
		t.Errorf("message() should stay single-part without HTML\ngot:\n%s", got)
	}
}
