package notify

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// The mails this system sends, written once here rather than assembled
// at each call site. They live in notify because every one of them is an
// outbound message and the callers already depend on this package for
// delivery; keeping the wording next to the transport is what stops a
// second "验证码" mail growing a different voice from the first.
//
// The HTML is deliberately old: tables, inline styles, no web fonts and
// no external images. Mail clients are a decade behind browsers -- Outlook
// still renders through Word -- and a layout that depends on anything
// newer arrives broken at exactly the recipients who matter.
//
// The mark is absent here on purpose. It is a drawing, and mail has no
// way to carry one: inline SVG does not render in Outlook, and a remote
// image is blocked by default in most clients, so either choice puts an
// empty box where the eye lands first. The header is the name against a
// brand-coloured rule instead -- a border is the one graphic every client
// draws.
const (
	brandColor  = "#6366f1"
	inkColor    = "#16233a"
	mutedColor  = "#6b7891"
	groundColor = "#f4f6fa"
	lineColor   = "#e5e9f0"
)

// Brand is the part of a mail a deployment may set. Only wording: the
// skeleton, the colours and the mark are ours, because they are what has
// to survive a decade-old mail client and because Fluxa is not
// white-labelled (DESIGN.md 6.1).
//
// Every field falls back, so an unconfigured deployment sends a complete
// mail rather than one with holes in it.
type Brand struct {
	// OrgName replaces "Fluxa" as who the mail is from, for a company
	// whose staff know the deployment by their own name.
	OrgName string
	// SignOff is the footer line under the rule.
	SignOff string
	// Contact tells the reader who to ask when something is wrong. It is
	// the single most useful thing to customise: the default can only say
	// "your administrator", which is not an address anyone can write to.
	Contact string
}

func (b Brand) orgName() string {
	if b.OrgName == "" {
		return "Fluxa"
	}
	return b.OrgName
}

func (b Brand) signOff() string {
	if b.SignOff == "" {
		return "Fluxa 企业内部 AI 资源分发管理系统"
	}
	return b.SignOff
}

// OTPMail is the verification code sent for local-account sign-in and
// registration.
func OTPMail(code string, ttl time.Duration, brand Brand) Mail {
	minutes := int(ttl.Minutes())
	org := brand.orgName()
	notes := []string{
		fmt.Sprintf("请勿把这个验证码转发给任何人，%s 的管理员也不会向你索要。", org),
		"如果这不是你本人的操作，忽略这封邮件即可，你的账号不会有任何变化。",
	}
	if brand.Contact != "" {
		notes = append(notes, brand.Contact)
	}

	text := fmt.Sprintf(
		"你的 %s 验证码是 %s，%d 分钟内有效。\r\n\r\n"+
			"请勿把这个验证码转发给任何人。如果这不是你本人的操作，忽略这封邮件即可，你的账号不会有任何变化。\r\n",
		org, code, minutes)
	if brand.Contact != "" {
		text += "\r\n" + brand.Contact + "\r\n"
	}
	text += "\r\n— " + brand.signOff()

	return Mail{
		Subject: fmt.Sprintf("%s 是你的 %s 验证码", code, org),
		Text:    text,
		HTML: render(mailBody{
			Brand:    org,
			SignOff:  brand.signOff(),
			Title:    "验证码",
			Lead:     fmt.Sprintf("用下面的验证码继续登录 %s。", org),
			Code:     code,
			CodeNote: fmt.Sprintf("%d 分钟内有效", minutes),
			Notes:    notes,
		}),
	}
}

// TestMail is what the 短信 / 邮件配置 page sends to prove a channel
// works. It says what it is plainly: an admin who receives it needs to
// know it was triggered by hand, not by somebody signing in.
func TestMail(brand Brand) Mail {
	org := brand.orgName()
	return Mail{
		Subject: org + " 邮件通道测试",
		Text: fmt.Sprintf("这是一封来自 %s 的测试邮件。\r\n\r\n", org) +
			"收到它说明邮件通道的服务器地址、端口和凭证都是通的，本地账号的验证码可以从这里发出。\r\n\r\n" +
			"— " + brand.signOff(),
		HTML: render(mailBody{
			Brand:   org,
			SignOff: brand.signOff(),
			Title:   "邮件通道测试",
			Lead:    fmt.Sprintf("这是一封来自 %s 的测试邮件，由管理员在后台手动触发。", org),
			Notes: []string{
				"收到它说明邮件通道的服务器地址、端口和凭证都是通的，本地账号的验证码可以从这里发出。",
				"没有人因此登录或注册，这封邮件不代表任何账号操作。",
			},
		}),
	}
}

type mailBody struct {
	Brand    string
	SignOff  string
	Title    string
	Lead     string
	Code     string
	CodeNote string
	Notes    []string
}

var shell = template.Must(template.New("mail").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:` + groundColor + `;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:` + groundColor + `;padding:32px 16px;">
<tr><td align="center">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;background:#ffffff;border:1px solid ` + lineColor + `;border-radius:12px;">
    <tr><td style="padding:28px 28px 0 28px;">
      <table role="presentation" cellpadding="0" cellspacing="0"><tr>
        <td style="border-left:3px solid ` + brandColor + `;padding-left:9px;font:600 16px/1.2 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + inkColor + `;">{{.Brand}}</td>
      </tr></table>
    </td></tr>
    <tr><td style="padding:22px 28px 0 28px;font:700 21px/1.4 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + inkColor + `;">{{.Title}}</td></tr>
    <tr><td style="padding:10px 28px 0 28px;font:400 14px/1.75 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + mutedColor + `;">{{.Lead}}</td></tr>
    {{if .Code}}
    <tr><td style="padding:22px 28px 0 28px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#eef2ff;border-radius:10px;">
        <tr><td align="center" style="padding:18px 12px 6px 12px;font:700 30px/1.2 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:.22em;color:` + brandColor + `;">{{.Code}}</td></tr>
        <tr><td align="center" style="padding:0 12px 16px 12px;font:400 12px/1.6 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + mutedColor + `;">{{.CodeNote}}</td></tr>
      </table>
    </td></tr>
    {{end}}
    {{range .Notes}}
    <tr><td style="padding:16px 28px 0 28px;font:400 13px/1.8 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + mutedColor + `;">{{.}}</td></tr>
    {{end}}
    <tr><td style="padding:24px 28px 26px 28px;">
      <div style="height:1px;background:` + lineColor + `;font-size:0;line-height:0;">&nbsp;</div>
      <div style="padding-top:16px;font:400 12px/1.7 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:` + mutedColor + `;">
        {{.SignOff}}<br>这封邮件由系统自动发出，请勿直接回复。
      </div>
    </td></tr>
  </table>
</td></tr></table>
</body></html>`))

func render(b mailBody) string {
	var out strings.Builder
	// The template is a constant and the fields are a code and fixed
	// copy, so this cannot fail for any input this package produces --
	// and a mail that failed to render must not become an empty one.
	if err := shell.Execute(&out, b); err != nil {
		return b.Lead
	}
	return out.String()
}
