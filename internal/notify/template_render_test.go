package notify

import (
	"strings"
	"testing"
	"time"
)

// The HTML has to survive mail clients, which is a much older target than
// a browser. These assert the constraints that make it survive rather
// than the wording, which is free to change.
func TestOTPMailRendersForMailClients(t *testing.T) {
	m := OTPMail("246810", 5*time.Minute, Brand{})

	for _, want := range []string{"246810", "5 分钟内有效"} {
		if !strings.Contains(m.HTML, want) || !strings.Contains(m.Text, strings.TrimSuffix(want, "内有效")) {
			t.Errorf("both parts should carry %q", want)
		}
	}
	if !strings.Contains(m.Subject, "246810") {
		t.Errorf("Subject = %q, want the code in it so a client preview shows it", m.Subject)
	}
	for _, banned := range []string{"<img", "display:flex", "display:grid", "<style", "position:absolute"} {
		if strings.Contains(m.HTML, banned) {
			t.Errorf("HTML uses %q, which does not survive Outlook or blocked remote content", banned)
		}
	}
	if !strings.Contains(m.HTML, `role="presentation"`) {
		t.Error("layout tables should be marked presentational for screen readers")
	}
}

func TestTestMailIsNotAVerificationCode(t *testing.T) {
	// It goes to an admin who pressed a button; dressing it as a sign-in
	// code would train people to expect codes they did not ask for.
	m := TestMail(Brand{})
	if strings.Contains(m.Subject, "验证码") {
		t.Errorf("Subject = %q, should not claim to be a verification code", m.Subject)
	}
	if strings.Contains(m.HTML, "letter-spacing:.22em") {
		t.Error("the test mail should not render a code block")
	}
}

func TestBrandFallsBackFieldByField(t *testing.T) {
	// A half-filled row is the normal state -- an admin sets the contact
	// line and leaves the rest -- so each field falls back on its own
	// rather than the whole struct being all-or-nothing.
	m := OTPMail("246810", 5*time.Minute, Brand{Contact: "遇到问题请联系 it@corp.example"})
	if !strings.Contains(m.HTML, "Fluxa") {
		t.Error("an unset org name should still render the default")
	}
	if !strings.Contains(m.HTML, "it@corp.example") || !strings.Contains(m.Text, "it@corp.example") {
		t.Error("the contact line should reach both parts")
	}
}

func TestBrandReplacesTheDefaultWording(t *testing.T) {
	m := OTPMail("246810", 5*time.Minute, Brand{OrgName: "示例科技", SignOff: "示例科技 IT 部"})
	for _, part := range []string{m.Subject, m.Text, m.HTML} {
		if !strings.Contains(part, "示例科技") {
			t.Errorf("configured org name missing from %q", part[:min(60, len(part))])
		}
	}
	if strings.Contains(m.HTML, "Fluxa 企业内部") {
		t.Error("a configured sign-off should replace the default, not sit beside it")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
