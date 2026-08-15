import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { LoginLayout } from "@/layouts/login-layout"
import { Logo } from "@/components/shared/logo"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"

type View = "entry" | "login" | "register" | "pending"

// Real login/registration paths, matching what internal/user/handler.go
// implements: Feishu OAuth (a full-page redirect to Feishu's own hosted
// authorize page, so there is no separate Fluxa-hosted "scan QR" screen
// to build here -- Feishu renders that itself) and phone/email OTP (no
// password anywhere in the product, DESIGN.md 7.1). Registration is the
// local-account fallback for companies without a unified IM (DESIGN.md
// 8.3): open self-registration, pending admin approval.
export function LoginPage() {
  const navigate = useNavigate()
  const { refresh } = useAuth()
  const [view, setView] = useState<View>("entry")
  const [identifier, setIdentifier] = useState("")
  const [code, setCode] = useState("")
  const [name, setName] = useState("")
  const [codeSent, setCodeSent] = useState(false)
  const [busy, setBusy] = useState(false)
  // Which login paths are actually configured: an admin hasn't set up
  // Feishu or a notify channel on a brand new deployment, so the
  // corresponding button must not be offered as if it worked -- see
  // internal/user/handler.go authMethods. null while still loading, so
  // nothing renders (and nothing is clickable) until we actually know.
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
      const path = view === "register" ? "/api/auth/local/register/request-otp" : "/api/auth/local/login/request-otp"
      await api.post(path, { identifier })
      setCodeSent(true)
      toast.success("验证码已发送")
    } catch (err) {
      toast.error(err instanceof ApiError && err.key === "common.validation_failed" ? "本地账号登录未启用" : "发送失败，请检查手机号/邮箱")
    } finally {
      setBusy(false)
    }
  }

  const verifyLogin = async () => {
    setBusy(true)
    try {
      await api.post("/api/auth/local/login/verify", { identifier, code })
      await refresh()
      navigate("/", { replace: true })
    } catch {
      toast.error("验证码错误或已过期")
    } finally {
      setBusy(false)
    }
  }

  const verifyRegister = async () => {
    setBusy(true)
    try {
      const res = await api.post<{ status?: string }>("/api/auth/local/register/verify", { identifier, code, name })
      if (res?.status === "pending_review") {
        setView("pending")
        return
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
      <LoginLayout>
        <div className="flex flex-col items-center gap-3 text-center">
          <Logo size={36} />
          <p className="text-base font-semibold text-foreground">注册成功，等待审批</p>
          <p className="text-[12.5px] text-muted-foreground">管理员审批通过后，你就可以用这个账号登录了</p>
          <button className="mt-1 text-xs font-semibold text-primary" onClick={() => setView("entry")}>
            返回登录
          </button>
        </div>
      </LoginLayout>
    )
  }

  if (view === "login" || view === "register") {
    const isRegister = view === "register"
    return (
      <LoginLayout>
        <div className="flex flex-col items-center gap-3.5">
          <Logo size={36} />
          <p className="text-base font-semibold text-foreground">{isRegister ? "注册 Fluxa 账号" : "手机号 / 邮箱登录"}</p>

          {isRegister && (
            <div className="w-full">
              <Label className="mb-1.5 text-xs">姓名</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="你的姓名" />
            </div>
          )}
          <div className="w-full">
            <Label className="mb-1.5 text-xs">手机号 / 邮箱</Label>
            <Input value={identifier} onChange={(e) => setIdentifier(e.target.value)} placeholder="请输入手机号或邮箱" />
          </div>
          <div className="w-full">
            <Label className="mb-1.5 text-xs">验证码</Label>
            <div className="flex gap-2">
              <Input value={code} onChange={(e) => setCode(e.target.value)} placeholder="请输入验证码" />
              <Button type="button" variant="outline" disabled={busy || !identifier} onClick={() => void requestCode()}>
                {codeSent ? "重新获取" : "获取验证码"}
              </Button>
            </div>
          </div>
          <Button
            className="mt-1 w-full"
            disabled={busy || !code || (isRegister && !name)}
            onClick={() => void (isRegister ? verifyRegister() : verifyLogin())}
          >
            {isRegister ? "注册并登录" : "登录"}
          </Button>
          <button className="text-xs font-semibold text-primary" onClick={() => setView(isRegister ? "login" : "register")}>
            {isRegister ? "已有账号？去登录" : "没有账号？注册一个"}
          </button>
          <button className="text-xs font-semibold text-muted-foreground" onClick={() => setView("entry")}>
            返回其他登录方式
          </button>
        </div>
      </LoginLayout>
    )
  }

  const noMethodsConfigured = methods && !methods.feishu && !methods.local

  return (
    <LoginLayout>
      <div className="flex flex-col items-center gap-4">
        <Logo size={44} />
        <div className="text-center">
          <p className="text-[19px] font-bold text-foreground">登录 Fluxa</p>
          <p className="mt-1 text-[12.5px] text-muted-foreground">企业内部 AI 资源分发管理系统</p>
        </div>

        {!methods && <p className="text-xs text-muted-foreground">加载中…</p>}

        {noMethodsConfigured && (
          <p className="text-center text-[12.5px] text-muted-foreground">
            管理员还没有配置登录方式（飞书或短信/邮箱），请联系管理员完成配置后再试
          </p>
        )}

        {methods?.feishu && (
          <Button className="w-full" onClick={() => (window.location.href = "/api/auth/feishu/login")}>
            使用飞书登录
          </Button>
        )}

        {methods?.feishu && methods?.local && (
          <div className="flex w-full items-center gap-2.5 text-[11px] text-muted-foreground">
            <span className="h-px flex-1 bg-border" />或<span className="h-px flex-1 bg-border" />
          </div>
        )}

        {methods?.local && (
          <Button variant="outline" className="w-full" onClick={() => setView("login")}>
            手机号 / 邮箱登录
          </Button>
        )}
      </div>
    </LoginLayout>
  )
}
