import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { LoginShell } from "@/layouts/login-layout"
import { Icon } from "@/components/console/icon"
import { Brand, FluxaLogo } from "@/components/console/brand"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"

type View = "entry" | "login" | "register" | "pending"

// Login as the hi-fi design draws it: Feishu is the primary button,
// phone/email is the fallback for companies without a unified IM
// (DESIGN.md 8.3). Two deliberate departures from the mockup, both
// forced by what the backend actually does:
//
//   - There is no Fluxa-hosted "scan the QR" screen. Feishu OAuth is a
//     full-page redirect to Feishu's own authorize page, which renders
//     the QR itself; drawing our own would be a picture of a thing that
//     doesn't exist.
//   - Registration needs a name and its own OTP purpose, so the single
//     form grows one extra field and a mode switch rather than pretending
//     "first login registers you" is a single call.
export function LoginPage() {
  const navigate = useNavigate()
  const { refresh } = useAuth()
  const [view, setView] = useState<View>("entry")
  const [channel, setChannel] = useState<"phone" | "email">("phone")
  const [identifier, setIdentifier] = useState("")
  const [code, setCode] = useState("")
  const [name, setName] = useState("")
  const [codeSent, setCodeSent] = useState(false)
  const [busy, setBusy] = useState(false)
  // Which login paths are actually configured: on a fresh deployment an
  // admin has set up neither Feishu nor a notify channel, so the matching
  // button must not be offered as if it worked -- see authMethods in
  // internal/user/handler.go. null while loading, so nothing is clickable
  // until we know.
  const [methods, setMethods] = useState<{ feishu: boolean; local: boolean } | null>(null)

  useEffect(() => {
    api
      .get<{ needsSetup: boolean }>("/api/setup/status")
      .then((res) => {
        if (res.needsSetup) navigate("/setup", { replace: true })
      })
      .catch(() => {})
  }, [navigate])

  useEffect(() => {
    api
      .get<{ feishu: boolean; local: boolean }>("/api/auth/methods")
      .then(setMethods)
      .catch(() => setMethods({ feishu: false, local: false }))
  }, [])

  const requestCode = async () => {
    setBusy(true)
    try {
      const path =
        view === "register" ? "/api/auth/local/register/request-otp" : "/api/auth/local/login/request-otp"
      await api.post(path, { identifier })
      setCodeSent(true)
      toast.success("验证码已发送")
    } catch (err) {
      toast.error(
        err instanceof ApiError && err.key === "common.validation_failed"
          ? "本地账号登录未启用"
          : "发送失败，请检查手机号 / 邮箱",
      )
    } finally {
      setBusy(false)
    }
  }

  const submit = async () => {
    setBusy(true)
    try {
      if (view === "register") {
        const res = await api.post<{ status?: string }>("/api/auth/local/register/verify", {
          identifier,
          code,
          name,
        })
        if (res?.status === "pending_review") {
          setView("pending")
          return
        }
      } else {
        await api.post("/api/auth/local/login/verify", { identifier, code })
      }
      await refresh()
      navigate("/", { replace: true })
    } catch {
      toast.error("验证码错误或已过期")
    } finally {
      setBusy(false)
    }
  }

  if (view === "pending") {
    return (
      <LoginShell>
        <div className="cn-login-card">
          <div className="cn-login-brand">
            <FluxaLogo size={44} radius={13} />
            <div style={{ textAlign: "center" }}>
              <div className="cn-login-title">注册成功，等待审批</div>
              <div className="cn-login-sub">管理员通过后，这个账号就能登录并发起调用</div>
            </div>
          </div>
          <button className="cn-login-btn" onClick={() => setView("entry")}>
            <Icon name="arrow-left" size={16} />
            返回登录
          </button>
        </div>
      </LoginShell>
    )
  }

  if (view === "login" || view === "register") {
    const isRegister = view === "register"
    return (
      <LoginShell>
        <div className="cn-login-card">
          <button className="cn-login-back" onClick={() => setView("entry")}>
            <Icon name="arrow-left" size={14} />返回
          </button>

          <div className="cn-login-brand" style={{ marginBottom: 18 }}>
            <div style={{ textAlign: "center" }}>
              <div className="cn-login-title">{isRegister ? "注册 Fluxa 账号" : "手机号 / 邮箱登录"}</div>
              <div className="cn-login-sub">
                {isRegister ? "注册后需管理员审核，通过即可使用" : "验证码登录，全程不使用密码"}
              </div>
            </div>
          </div>

          <div className="cn-tabs" style={{ marginBottom: 16 }}>
            <button data-on={channel === "phone"} onClick={() => setChannel("phone")} style={{ flex: 1 }}>
              手机号
            </button>
            <button data-on={channel === "email"} onClick={() => setChannel("email")} style={{ flex: 1 }}>
              邮箱
            </button>
          </div>

          <div className="cn-form">
            {isRegister && (
              <div className="cn-form-row">
                <label className="cn-form-label" htmlFor="login-name">姓名</label>
                <input
                  id="login-name"
                  className="cn-input"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="你的姓名"
                />
              </div>
            )}

            <div className="cn-form-row">
              <label className="cn-form-label" htmlFor="login-id">{channel === "phone" ? "手机号" : "邮箱"}</label>
              <input
                id="login-id"
                className="cn-input"
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                placeholder={channel === "phone" ? "138 0000 0000" : "name@example.com"}
              />
            </div>

            <div className="cn-form-row">
              <label className="cn-form-label" htmlFor="login-code">验证码</label>
              <div className="cn-code-input">
                <input
                  id="login-code"
                  className="cn-input"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="6 位数字"
                />
                <button className="cn-code-send" disabled={busy || !identifier} onClick={() => void requestCode()}>
                  {codeSent ? "重新获取" : "获取验证码"}
                </button>
              </div>
              <div className="cn-input-hint">
                验证码由管理员配置的{channel === "phone" ? "短信" : "邮件"}通道发出
              </div>
            </div>

            <button
              className="cn-login-btn cn-login-btn-pri"
              style={{ marginTop: 2 }}
              disabled={busy || !code || !identifier || (isRegister && !name)}
              onClick={() => void submit()}
            >
              {isRegister ? "注册并登录" : "登录"}
            </button>
          </div>

          <div className="cn-login-foot">
            {isRegister ? "已有账号？" : "还没有账号？"}
            <a
              href="#"
              onClick={(e) => {
                e.preventDefault()
                setCode("")
                setCodeSent(false)
                setView(isRegister ? "login" : "register")
              }}
            >
              {isRegister ? "去登录" : "注册一个"}
            </a>
          </div>

          {isRegister && (
            <div style={{ marginTop: 16 }}>
              <div className="cn-notice">
                <Icon name="shield-check" size={14} />
                <span>新账号注册后处于「待审核」状态，管理员通过后才能发起调用。</span>
              </div>
            </div>
          )}
        </div>
      </LoginShell>
    )
  }

  const nothingConfigured = methods && !methods.feishu && !methods.local

  return (
    <LoginShell>
      <div className="cn-login-card">
        <div className="cn-login-brand">
          <FluxaLogo size={44} radius={13} />
          <div style={{ textAlign: "center" }}>
            <div className="cn-login-title">登录 Fluxa</div>
            <div className="cn-login-sub">企业内部 AI 资源分发管理系统</div>
          </div>
        </div>

        {!methods && <div className="cn-login-sub">加载中…</div>}

        {nothingConfigured && (
          <div className="cn-notice">
            <Icon name="alert-triangle" size={14} />
            <span>管理员还没有配置任何登录方式（飞书或短信 / 邮件通道），请联系管理员完成配置后再试。</span>
          </div>
        )}

        {methods?.feishu && (
          <div className="cn-login-btns">
            <button
              className="cn-login-btn cn-login-btn-pri"
              onClick={() => (window.location.href = "/api/auth/feishu/login")}
            >
              <Brand kind="feishu" size={22} />
              使用飞书登录
            </button>
          </div>
        )}

        {methods?.feishu && methods?.local && <div className="cn-login-or">其他方式</div>}

        {methods?.local && (
          <div className="cn-login-btns">
            <button className="cn-login-btn" onClick={() => setView("login")}>
              <Icon name="smartphone" size={16} />
              手机号 / 邮箱登录
            </button>
          </div>
        )}

        <div className="cn-login-foot">
          首次使用飞书登录会自动创建账号并同步部门。
          <br />
          遇到问题请联系管理员。
        </div>
      </div>
    </LoginShell>
  )
}
