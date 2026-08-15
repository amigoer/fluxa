import { useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { LoginLayout } from "@/layouts/login-layout"
import { Logo } from "@/components/shared/logo"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"

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

  return (
    <LoginLayout>
      <div className="flex flex-col items-center gap-4">
        <Logo size={44} />
        <div className="text-center">
          <p className="text-[19px] font-bold text-foreground">初始化 Fluxa</p>
          <p className="mt-1 text-[12.5px] text-muted-foreground">首次部署，创建企业和第一个超管账号</p>
        </div>

        <form className="mt-2 flex w-full flex-col gap-3.5" onSubmit={onSubmit}>
          <div>
            <Label className="mb-1.5 text-xs">企业名称</Label>
            <Input value={orgName} onChange={(e) => setOrgName(e.target.value)} required placeholder="例如：某某科技有限公司" />
          </div>
          <div>
            <Label className="mb-1.5 text-xs">你的姓名</Label>
            <Input value={adminName} onChange={(e) => setAdminName(e.target.value)} required placeholder="超管账号姓名" />
          </div>
          <div>
            <Label className="mb-1.5 text-xs">邮箱</Label>
            <Input type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} required placeholder="用于登录和通知" />
          </div>
          <Button type="submit" disabled={submitting} className="mt-1">
            {submitting ? "创建中…" : "创建并进入"}
          </Button>
        </form>
      </div>
    </LoginLayout>
  )
}
