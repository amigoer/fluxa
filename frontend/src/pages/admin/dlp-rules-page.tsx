import { useState } from "react"
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
import type { DLPRule } from "@/lib/types"

const matchTypeLabel = { regex_checksum: "正则 + 校验位", keyword: "关键词" }
const actionLabel = { mask: "脱敏", block: "拦截" }

export function DlpRulesPage() {
  const { data: rules, refetch } = useApiQuery<DLPRule[]>("/api/dlp-rules")
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [matchType, setMatchType] = useState<"regex_checksum" | "keyword">("keyword")
  const [pattern, setPattern] = useState("")
  const [action, setAction] = useState<"mask" | "block">("block")
  const [priority, setPriority] = useState("100")

  const create = async () => {
    try {
      await api.post("/api/dlp-rules", { Name: name, MatchType: matchType, Pattern: pattern, Action: action, Priority: Number(priority), Enabled: true })
      setOpen(false)
      setName("")
      setPattern("")
      refetch()
      toast.success("规则已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  const toggle = async (id: string, enabled: boolean) => {
    await api.patch(`/api/dlp-rules/${id}/enabled`, { enabled })
    refetch()
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="DLP 规则" />

      <div className="flex items-center gap-2">
        <Input placeholder="搜索规则…" className="max-w-[260px]" />
        <div className="flex-1" />
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>新建规则</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>新建规则</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div>
                <Label className="mb-1.5 text-xs">规则名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">匹配方式</Label>
                <Select value={matchType} onValueChange={(v) => setMatchType(v as typeof matchType)}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="keyword">关键词</SelectItem>
                    <SelectItem value="regex_checksum">内置校验位类型（id_card / bank_card / phone）</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="mb-1.5 text-xs">{matchType === "keyword" ? "关键词（逗号分隔）" : "内置类型标识"}</Label>
                <Input value={pattern} onChange={(e) => setPattern(e.target.value)} placeholder={matchType === "keyword" ? "机密,内部,禁止外传" : "id_card"} />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">命中动作</Label>
                <Select value={action} onValueChange={(v) => setAction(v as typeof action)}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="mask">脱敏</SelectItem>
                    <SelectItem value="block">拦截</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="mb-1.5 text-xs">优先级（数字越小越先执行）</Label>
                <Input value={priority} onChange={(e) => setPriority(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button disabled={!name || !pattern} onClick={() => void create()}>
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
              <th className="p-3 font-semibold">规则名称</th>
              <th className="p-3 font-semibold">匹配方式</th>
              <th className="p-3 font-semibold">动作</th>
              <th className="p-3 font-semibold">优先级</th>
              <th className="p-3 font-semibold">状态</th>
            </tr>
          </thead>
          <tbody>
            {(rules ?? []).map((r) => (
              <tr key={r.ID} className="border-t border-border">
                <td className="p-3 text-foreground">{r.Name}</td>
                <td className="p-3 text-muted-foreground">{matchTypeLabel[r.MatchType]}</td>
                <td className="p-3">
                  <StatusPill tone={r.Action === "block" ? "bad" : "ok"}>{actionLabel[r.Action]}</StatusPill>
                </td>
                <td className="p-3 text-muted-foreground">{r.Priority}</td>
                <td className="p-3">
                  <Switch checked={r.Enabled} onCheckedChange={(v) => void toggle(r.ID, v)} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(rules ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无规则</p>}
      </div>
    </div>
  )
}
