import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
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
import type { Model, RoutingRule } from "@/lib/types"

export function MyRoutingPage() {
  const { data: rules, refetch } = useApiQuery<RoutingRule[]>("/api/routing/mine")
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const modelName = (id: string | null) => (models ?? []).find((m) => m.ID === id)?.Name ?? "—"

  const [open, setOpen] = useState(false)
  const [condition, setCondition] = useState("默认")
  const [targetModelId, setTargetModelId] = useState("")
  const [fallbackModelId, setFallbackModelId] = useState("")
  const [costCeiling, setCostCeiling] = useState("")

  const create = async () => {
    try {
      await api.post("/api/routing/mine", {
        ConditionLabel: condition,
        TargetModelID: targetModelId,
        FallbackModelID: fallbackModelId || null,
        CostCeilingCents: costCeiling ? Math.round(Number(costCeiling) * 100) : null,
      })
      setOpen(false)
      refetch()
      toast.success("路由规则已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="我的路由配置"
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>添加规则</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>添加个人路由规则</DialogTitle>
              </DialogHeader>
              <div className="flex flex-col gap-3.5">
                <div>
                  <Label className="mb-1.5 text-xs">条件</Label>
                  <Input value={condition} onChange={(e) => setCondition(e.target.value)} placeholder="默认 / 代码类任务 / 长文本（>50K）" />
                </div>
                <div>
                  <Label className="mb-1.5 text-xs">目标模型</Label>
                  <Select value={targetModelId} onValueChange={setTargetModelId}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="选择模型" />
                    </SelectTrigger>
                    <SelectContent>
                      {(models ?? []).map((m) => (
                        <SelectItem key={m.ID} value={m.ID}>
                          {m.Name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label className="mb-1.5 text-xs">备用模型（可选）</Label>
                  <Select value={fallbackModelId} onValueChange={setFallbackModelId}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="不设置" />
                    </SelectTrigger>
                    <SelectContent>
                      {(models ?? []).map((m) => (
                        <SelectItem key={m.ID} value={m.ID}>
                          {m.Name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label className="mb-1.5 text-xs">备用模型成本上限（¥，可选）</Label>
                  <Input value={costCeiling} onChange={(e) => setCostCeiling(e.target.value)} placeholder="不设置则无上限" />
                </div>
              </div>
              <DialogFooter>
                <Button disabled={!targetModelId} onClick={() => void create()}>
                  创建
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <div className="flex flex-col">
          {(rules ?? []).map((r) => (
            <div key={r.ID} className="flex flex-wrap items-center gap-2 border-t border-border py-3 text-xs first:border-t-0">
              <span className="rounded-md border border-dashed border-border px-2.5 py-1.5 text-muted-foreground">{r.ConditionLabel}</span>
              <span className="text-muted-foreground">→</span>
              <span className="rounded-md border border-border bg-background px-2.5 py-1.5 text-foreground">{modelName(r.TargetModelID)}</span>
              {r.FallbackModelID && (
                <>
                  <span className="text-muted-foreground">→</span>
                  <span className="rounded-md border border-border bg-background px-2.5 py-1.5 text-foreground">
                    {modelName(r.FallbackModelID)}（备用）
                  </span>
                </>
              )}
            </div>
          ))}
          {(rules ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">还没有配置个人路由，将使用全局默认路由</p>}
        </div>
      </div>
    </div>
  )
}
