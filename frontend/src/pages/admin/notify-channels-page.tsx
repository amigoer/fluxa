import { useEffect, useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface ChannelResponse {
  channel: { Provider: string; Config: Record<string, string>; Enabled: boolean }
  sentThisMonth: number
}

import { api } from "@/lib/api"

function SmsCard() {
  const [data, setData] = useState<ChannelResponse | null>(null)
  const [open, setOpen] = useState(false)
  const [accessKeyId, setAccessKeyId] = useState("")
  const [accessKeySecret, setAccessKeySecret] = useState("")
  const [signName, setSignName] = useState("")
  const [templateCode, setTemplateCode] = useState("")

  const load = () => api.get<ChannelResponse>("/api/notify-channels/sms").then(setData)
  useEffect(() => {
    load()
  }, [])

  const save = async (enabled: boolean) => {
    try {
      await api.put("/api/notify-channels/sms", {
        Provider: "aliyun_sms",
        Config: { access_key_id: accessKeyId, access_key_secret: accessKeySecret, sign_name: signName, template_code: templateCode },
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
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-[12.5px] font-bold text-accent-foreground">短</span>
          <span className="text-[13px] font-semibold text-foreground">短信</span>
        </span>
        <StatusPill tone={data?.channel.Enabled ? "ok" : "warn"}>{data?.channel.Enabled ? "已启用" : "未配置"}</StatusPill>
      </div>
      <div className="flex flex-col gap-1.5 text-[11.5px]">
        <Row k="服务商" v={data?.channel.Provider || "—"} />
        <Row k="本月已发送" v={`${data?.sentThisMonth ?? 0} 条`} />
      </div>
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <Button variant="link" className="h-auto p-0 text-[11.5px]" onClick={() => setOpen(true)}>
            {data?.channel.Enabled ? "编辑" : "配置"}
          </Button>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>配置短信通道（阿里云短信）</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div>
                <Label className="mb-1.5 text-xs">AccessKey ID</Label>
                <Input value={accessKeyId} onChange={(e) => setAccessKeyId(e.target.value)} />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">AccessKey Secret</Label>
                <Input value={accessKeySecret} onChange={(e) => setAccessKeySecret(e.target.value)} type="password" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">签名</Label>
                <Input value={signName} onChange={(e) => setSignName(e.target.value)} placeholder="【Fluxa】" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">模板 ID</Label>
                <Input value={templateCode} onChange={(e) => setTemplateCode(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button disabled={!accessKeyId || !accessKeySecret} onClick={() => void save(true)}>
                保存并启用
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}

function EmailCard() {
  const [data, setData] = useState<ChannelResponse | null>(null)
  const [open, setOpen] = useState(false)
  const [host, setHost] = useState("")
  const [port, setPort] = useState("465")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [fromAddress, setFromAddress] = useState("")

  const load = () => api.get<ChannelResponse>("/api/notify-channels/email").then(setData)
  useEffect(() => {
    load()
  }, [])

  const save = async (enabled: boolean) => {
    try {
      await api.put("/api/notify-channels/email", {
        Provider: "smtp",
        Config: { host, port, username, password, from_address: fromAddress },
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
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent text-[12.5px] font-bold text-accent-foreground">邮</span>
          <span className="text-[13px] font-semibold text-foreground">邮件（SMTP）</span>
        </span>
        <StatusPill tone={data?.channel.Enabled ? "ok" : "warn"}>{data?.channel.Enabled ? "已启用" : "未配置"}</StatusPill>
      </div>
      <div className="flex flex-col gap-1.5 text-[11.5px]">
        <Row k="SMTP 服务器" v={data?.channel.Config?.host || "—"} />
        <Row k="本月已发送" v={`${data?.sentThisMonth ?? 0} 封`} />
      </div>
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <Button variant="link" className="h-auto p-0 text-[11.5px]" onClick={() => setOpen(true)}>
            {data?.channel.Enabled ? "编辑" : "配置"}
          </Button>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>配置邮件通道（SMTP）</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label className="mb-1.5 text-xs">SMTP 服务器</Label>
                  <Input value={host} onChange={(e) => setHost(e.target.value)} placeholder="smtp.example.com" />
                </div>
                <div>
                  <Label className="mb-1.5 text-xs">端口</Label>
                  <Input value={port} onChange={(e) => setPort(e.target.value)} />
                </div>
              </div>
              <div>
                <Label className="mb-1.5 text-xs">发件账号</Label>
                <Input value={username} onChange={(e) => setUsername(e.target.value)} />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">密码 / 授权码</Label>
                <Input value={password} onChange={(e) => setPassword(e.target.value)} type="password" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">发件人地址</Label>
                <Input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button disabled={!host || !fromAddress} onClick={() => void save(true)}>
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
    <div className="flex justify-between gap-2.5">
      <span className="text-muted-foreground">{k}</span>
      <span className="truncate font-mono text-foreground">{v}</span>
    </div>
  )
}

export function NotifyChannelsPage() {
  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="短信 / 邮件配置" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <SmsCard />
        <EmailCard />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <span className="text-[12.5px] text-foreground">
          发信通道用途
          <span className="mt-0.5 block text-[11.5px] text-muted-foreground">
            本地账号注册/登录的验证码，从这里配置的通道发出，不写死具体服务商
          </span>
        </span>
      </div>
    </div>
  )
}
