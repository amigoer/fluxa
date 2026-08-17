import { useEffect, useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { LoginShell } from "@/layouts/login-layout"
import { FluxaLogo } from "@/components/console/brand"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { Input } from "@/components/console/ui"

// First-run setup: creates the single organization this deployment
// serves and its first super admin (DESIGN.md 9, one org per
// deployment). Only reachable while no organization exists yet --
// backend rejects it otherwise.
export function SetupPage() {
  const navigate = useNavigate()
  const { refresh } = useAuth()
  const [orgName, setOrgName] = useState("")
  const [adminName, setAdminName] = useState("")
  const [adminEmail, setAdminEmail] = useState("")
  const [submitting, setSubmitting] = useState(false)
  // The backend rejects a second setup with 409, but nothing stopped an
  // already-configured deployment from *rendering* this form -- landing
  // on /setup directly showed a full 初始化 Fluxa form whose only
  // possible outcome was that error. The status check LoginPage already
  // makes is simply made in the other direction here, and the form is
  // held back until the answer is in so it never flashes.
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null)

  useEffect(() => {
    api
      .get<{ needsSetup: boolean }>("/api/setup/status")
      .then((res) => {
        setNeedsSetup(res.needsSetup)
        if (!res.needsSetup) navigate("/login", { replace: true })
      })
      // An unreachable backend is not evidence that setup is done; show
      // the form and let the submit surface the real failure.
      .catch(() => setNeedsSetup(true))
  }, [navigate])

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    try {
      await api.post("/api/setup", { orgName, adminName, adminEmail })
      // The setup call sets a session cookie, but AuthProvider only
      // fetches /api/me once on mount -- without this, Landing would
      // still see the pre-setup "logged out" state and bounce back to
      // /login (this was a real bug, caught by clicking through the
      // flow rather than just typechecking it).
      await refresh()
      navigate("/", { replace: true })
    } catch {
      toast.error("初始化失败，请检查填写内容")
    } finally {
      setSubmitting(false)
    }
  }

  if (needsSetup !== true) return null

  return (
    <LoginShell>
      <div>
        <div className="cn-auth-brand">
          <FluxaLogo size={38} radius={11} />
          <h1 className="cn-auth-title">初始化 Fluxa</h1>
          <p className="cn-auth-sub">首次部署，创建企业和第一个超管账号</p>
        </div>

        <form className="cn-form" onSubmit={onSubmit}>
          <div className="cn-form-row">
            <label className="cn-form-label" htmlFor="setup-org">
              企业名称 <span>必填</span>
            </label>
            <Input
              id="setup-org"
              value={orgName}
              onChange={(e) => setOrgName(e.target.value)}
              required
              placeholder="例如：某某科技有限公司"
            />
          </div>
          <div className="cn-form-row">
            <label className="cn-form-label" htmlFor="setup-name">
              你的姓名 <span>必填</span>
            </label>
            <Input
              id="setup-name"
              value={adminName}
              onChange={(e) => setAdminName(e.target.value)}
              required
              placeholder="超管账号姓名"
            />
          </div>
          <div className="cn-form-row">
            <label className="cn-form-label" htmlFor="setup-email">
              邮箱 <span>必填</span>
            </label>
            <Input
              id="setup-email"
              type="email"
              value={adminEmail}
              onChange={(e) => setAdminEmail(e.target.value)}
              required
              placeholder="用于登录和通知"
            />
            <div className="cn-input-hint">这个邮箱同时是超管账号的登录标识。</div>
          </div>

          <button className="cn-login-btn cn-login-btn-pri" type="submit" disabled={submitting}>
            {submitting ? "创建中…" : "创建并进入"}
          </button>
        </form>
      </div>
    </LoginShell>
  )
}
