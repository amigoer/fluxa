import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { ProviderAvatar } from "@/components/shared/provider-avatar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import type { Model, Provider } from "@/lib/types"

const kindLabel: Record<string, string> = {
  openai_compatible: "OpenAI 兼容",
  anthropic: "Anthropic",
  azure_openai: "Azure OpenAI",
  gemini: "Gemini",
  bedrock: "Bedrock",
}

export function ProvidersPage() {
  const { data: providers, refetch } = useApiQuery<Provider[]>("/api/providers")
  const { data: models } = useApiQuery<Model[]>("/api/models")
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [kind, setKind] = useState("openai_compatible")
  const [baseUrl, setBaseUrl] = useState("")
  const [apiKey, setApiKey] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const modelCount = (providerId: string) => (models ?? []).filter((m) => m.ProviderID === providerId).length

  const create = async () => {
    setSubmitting(true)
    try {
      await api.post("/api/providers", { Name: name, Kind: kind, Config: { base_url: baseUrl, api_key: apiKey } })
      setOpen(false)
      setName("")
      setBaseUrl("")
      setApiKey("")
      refetch()
      toast.success("供应商已创建")
    } catch {
      toast.error("创建失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="供应商" />

      <div className="flex items-center gap-2">
        <Input placeholder="搜索供应商…" className="max-w-[260px]" />
        <div className="flex-1" />
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>新建供应商</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>新建供应商</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div>
                <Label className="mb-1.5 text-xs">名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：OpenAI Production" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">类型</Label>
                <Select value={kind} onValueChange={setKind}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {Object.entries(kindLabel).map(([value, label]) => (
                      <SelectItem key={value} value={value}>
                        {label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="mb-1.5 text-xs">Base URL</Label>
                <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">API Key</Label>
                <Input value={apiKey} onChange={(e) => setApiKey(e.target.value)} type="password" />
              </div>
            </div>
            <DialogFooter>
              <Button disabled={submitting || !name} onClick={() => void create()}>
                创建
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">名称</th>
              <th className="p-3 font-semibold">类型</th>
              <th className="p-3 font-semibold">状态</th>
              <th className="p-3 font-semibold">模型数</th>
            </tr>
          </thead>
          <tbody>
            {(providers ?? []).map((p) => (
              <tr key={p.ID} className="border-t border-border">
                <td className="p-3">
                  <span className="flex items-center text-foreground">
                    <ProviderAvatar name={p.Name} />
                    {p.Name}
                  </span>
                </td>
                <td className="p-3 text-muted-foreground">{kindLabel[p.Kind] ?? p.Kind}</td>
                <td className="p-3">
                  <StatusPill tone={p.Status === "active" ? "ok" : "bad"}>
                    {p.Status === "active" ? "正常" : "已停用"}
                  </StatusPill>
                </td>
                <td className="p-3 text-muted-foreground">{modelCount(p.ID)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(providers ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无供应商</p>}
      </div>
    </div>
  )
}
