import { useEffect, useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { api } from "@/lib/api"
import type { AuthSettings, IdentityConfig } from "@/lib/types"

const providers: { key: string; label: string; letter: string }[] = [
  { key: "feishu", label: "飞书", letter: "飞" },
  { key: "wecom", label: "企业微信", letter: "企" },
  { key: "dingtalk", label: "钉钉", letter: "钉" },
]

function IdentityCard({ providerKey, label, letter }: { providerKey: string; label: string; letter: string }) {
  const [config, setConfig] = useState<IdentityConfig | null>(null)
  const [open, setOpen] = useState(false)
  const [appId, setAppId] = useState("")
  const [appSecret, setAppSecret] = useState("")
  const [callbackPath, setCallbackPath] = useState(`/api/auth/${providerKey}/callback`)

  const load = () => {
    api.get<IdentityConfig>(`/api/identity-configs/${providerKey}`).then(setConfig)
  }
  useEffect(load, [providerKey])

  const save = async (enabled: boolean) => {
    try {
      await api.put(`/api/identity-configs/${providerKey}`, {
        AppID: appId,
        AppSecret: appSecret,
        CallbackPath: callbackPath,
        Enabled: enabled,
      })
      setOpen(false)
      load()
      toast.success("已保存")
    } catch {
      toast.error("保存失败")
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2.5">
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-[12.5px] font-bold text-accent-foreground">
            {letter}
          </span>
          <span className="text-[13px] font-semibold text-foreground">{label}</span>
        </span>
        <StatusPill tone={config?.Enabled ? "ok" : "warn"}>{config?.Enabled ? "已启用" : "未配置"}</StatusPill>
      </div>
      <div className="flex flex-col gap-1.5">
        <Row k="App ID" v={config?.AppID || "—"} />
        <Row k="回调地址" v={config?.CallbackPath || callbackPath} />
      </div>
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <Button variant="link" className="h-auto p-0 text-[11.5px]" onClick={() => setOpen(true)}>
            {config?.Enabled ? "编辑" : "配置"}
          </Button>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>配置{label}</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div>
                <Label className="mb-1.5 text-xs">App ID</Label>
                <Input value={appId} onChange={(e) => setAppId(e.target.value)} />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">App Secret</Label>
                <Input value={appSecret} onChange={(e) => setAppSecret(e.target.value)} type="password" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">回调路径</Label>
                <Input value={callbackPath} onChange={(e) => setCallbackPath(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => void save(false)}>
                保存草稿
              </Button>
              <Button disabled={!appId || !appSecret} onClick={() => void save(true)}>
                保存并启用
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-2.5 text-[11.5px]">
      <span className="text-muted-foreground">{k}</span>
      <span className="truncate font-mono text-foreground">{v}</span>
    </div>
  )
}

function LocalAccountCard() {
  const [settings, setSettings] = useState<AuthSettings | null>(null)

  const load = () => {
    api.get<AuthSettings>("/api/auth-settings").then(setSettings)
  }
  useEffect(load, [])

  const update = async (patch: Partial<AuthSettings>) => {
    if (!settings) return
    const next = { ...settings, ...patch }
    setSettings(next)
    await api.put("/api/auth-settings", next)
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2.5">
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-[12.5px] font-bold text-accent-foreground">兜</span>
          <span className="text-[13px] font-semibold text-foreground">本地账号兜底</span>
        </span>
        <StatusPill tone={settings?.LocalAccountEnabled ? "ok" : "warn"}>
          {settings?.LocalAccountEnabled ? "已启用" : "已关闭"}
        </StatusPill>
      </div>
      <div className="flex items-center justify-between text-[12.5px] text-foreground">
        启用本地账号注册/登录
        <Switch checked={settings?.LocalAccountEnabled ?? false} onCheckedChange={(v) => void update({ LocalAccountEnabled: v })} />
      </div>
      <div className="flex items-center justify-between text-[12.5px] text-foreground">
        注册后需管理员审批
        <Switch
          checked={settings?.LocalAccountRequiresApproval ?? true}
          onCheckedChange={(v) => void update({ LocalAccountRequiresApproval: v })}
        />
      </div>
    </div>
  )
}

export function IdentitySourcesPage() {
  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="身份源" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {providers.map((p) => (
          <IdentityCard key={p.key} providerKey={p.key} label={p.label} letter={p.letter} />
        ))}
        <LocalAccountCard />
      </div>
    </div>
  )
}
