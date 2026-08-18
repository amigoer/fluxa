import { useEffect, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Field, Input } from "@/components/console/ui"
import { api } from "@/lib/api"

interface MailSettings {
  OrgName: string
  SignOff: string
  ContactLine: string
}

const EMPTY: MailSettings = { OrgName: "", SignOff: "", ContactLine: "" }

// Editing the mail's wording, not its markup. The skeleton has to survive
// mail clients a decade behind browsers, and an admin editing HTML with
// no rendering target breaks that silently -- so the fields are the parts
// that are safe to vary, and the preview beside them is the real
// template rendered by the same code that sends it.
export function MailTemplatePage() {
  const navigate = useNavigate()
  const [form, setForm] = useState<MailSettings>(EMPTY)
  const [saved, setSaved] = useState<MailSettings | null>(null)
  const [html, setHtml] = useState("")
  const [busy, setBusy] = useState(false)
  const frame = useRef<HTMLIFrameElement>(null)

  useEffect(() => {
    api
      .get<MailSettings>("/api/mail-settings")
      .then((res) => {
        setForm(res)
        setSaved(res)
      })
      .catch(() => toast.error("读取失败"))
  }, [])

  // Re-rendered from the values in the form rather than from what is
  // stored, so the preview answers "what will this look like" before
  // saving rather than after.
  useEffect(() => {
    if (saved === null) return
    const t = setTimeout(() => {
      api
        .postText("/api/mail-settings/preview", form)
        .then(setHtml)
        .catch(() => setHtml(""))
    }, 250)
    return () => clearTimeout(t)
  }, [form, saved])

  // Written after the element is in the document, not through JSX. React
  // assigns srcDoc while the iframe is still detached, and an iframe with
  // no browsing context has nothing to load into -- the frame then sits
  // on the empty document it got at insertion and never shows the
  // preview. Assigning here happens post-mount, where it navigates.
  useEffect(() => {
    if (frame.current) frame.current.srcdoc = html
  }, [html])

  const dirty = useMemo(
    () => saved !== null && JSON.stringify(form) !== JSON.stringify(saved),
    [form, saved],
  )

  const save = async () => {
    setBusy(true)
    try {
      await api.put("/api/mail-settings", form)
      setSaved(form)
      toast.success("已保存，下一封邮件即生效")
    } catch {
      toast.error("保存失败")
    } finally {
      setBusy(false)
    }
  }

  const set = (k: keyof MailSettings) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

  return (
    <div className="cn-page">
      <div className="cn-page-head">
        <div>
          <button className="cn-login-back" onClick={() => navigate("/admin/notify-channels")}>
            <Icon name="arrow-left" size={14} />
            短信 / 邮件配置
          </button>
          <h1 className="cn-page-title">邮件模板</h1>
          <p className="cn-page-sub">验证码和通道测试邮件共用这套文案，右侧是真实渲染结果</p>
        </div>
        <div className="cn-page-acts">
          <Button tone="primary" disabled={!dirty || busy} onClick={() => void save()}>
            保存
          </Button>
        </div>
      </div>

      <div className="cn-mailtpl">
        <Card title="可改的部分" note="留空即使用内置文案">
          <div className="cn-card-body">
            <div className="cn-form">
              <Field label="企业名称" hint="出现在邮件抬头、标题和正文里，替换默认的「Fluxa」">
                <Input value={form.OrgName} onChange={set("OrgName")} placeholder="Fluxa" />
              </Field>
              <Field label="落款" hint="正文下方那条分隔线之后的一行">
                <Input
                  value={form.SignOff}
                  onChange={set("SignOff")}
                  placeholder="Fluxa 企业内部 AI 资源分发管理系统"
                />
              </Field>
              <Field label="求助方式" hint="默认文案只能说「联系管理员」，这里可以给出真正能写信的地址">
                <Input
                  value={form.ContactLine}
                  onChange={set("ContactLine")}
                  placeholder="遇到问题请联系 it@example.com"
                />
              </Field>
            </div>

            <div className="cn-notice" style={{ marginTop: 14 }}>
              <Icon name="shield-check" size={14} />
              <span>
                版式、配色和标记不开放修改：邮件客户端比浏览器落后很多，自定义 HTML
                会在收件人那里静默变形；品牌视觉也不做二次定制（见 DESIGN.md 6.1）。
              </span>
            </div>
          </div>
        </Card>

        <Card title="预览" note="验证码邮件 · 示例码 000000">
          <div className="cn-card-body">
            {/* Sandboxed with nothing granted: it renders a document
                assembled from admin input, and it has no reason to run
                scripts or reach anything. */}
            <iframe ref={frame} className="cn-mailtpl-frame" title="邮件预览" sandbox="" />
          </div>
        </Card>
      </div>
    </div>
  )
}
