import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { ProviderAvatar } from "@/components/shared/provider-avatar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
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
import { formatCents } from "@/lib/format"
import type { Model, Provider, RoutingRule } from "@/lib/types"

export function ModelsRoutingPage() {
  const { data: models, refetch: refetchModels } = useApiQuery<Model[]>("/api/models")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const { data: rules, refetch: refetchRules } = useApiQuery<RoutingRule[]>("/api/routing/global")

  const providerName = (id: string) => (providers ?? []).find((p) => p.ID === id)?.Name ?? id
  const providerKind = (id: string) => (providers ?? []).find((p) => p.ID === id)?.Kind
  const modelName = (id: string | null) => (models ?? []).find((m) => m.ID === id)?.Name ?? "—"

  const [modelOpen, setModelOpen] = useState(false)
  const [mName, setMName] = useState("")
  const [mIdentifier, setMIdentifier] = useState("")
  const [mProviderId, setMProviderId] = useState("")
  const [mInputPrice, setMInputPrice] = useState("")
  const [mOutputPrice, setMOutputPrice] = useState("")

  const createModel = async () => {
    try {
      await api.post("/api/models", {
        ProviderID: mProviderId,
        Name: mName,
        ModelIdentifier: mIdentifier,
        Status: "published",
        InputPriceCentsPer1M: Math.round(Number(mInputPrice) * 100),
        OutputPriceCentsPer1M: Math.round(Number(mOutputPrice) * 100),
        ContextWindow: 128000,
      })
      setModelOpen(false)
      setMName("")
      setMIdentifier("")
      refetchModels()
      toast.success("模型已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  const [ruleOpen, setRuleOpen] = useState(false)
  const [condition, setCondition] = useState("默认")
  const [targetModelId, setTargetModelId] = useState("")
  const [fallbackModelId, setFallbackModelId] = useState("")

  const createRule = async () => {
    try {
      await api.post("/api/routing/global", {
        ConditionLabel: condition,
        TargetModelID: targetModelId,
        FallbackModelID: fallbackModelId || null,
      })
      setRuleOpen(false)
      refetchRules()
      toast.success("路由规则已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="模型与路由" />

      <Tabs defaultValue="models">
        <div className="flex items-center justify-between">
          <TabsList>
            <TabsTrigger value="models">模型目录</TabsTrigger>
            <TabsTrigger value="routing">路由策略</TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="models" className="mt-3.5 flex flex-col gap-3.5">
          <div className="flex justify-end">
            <Dialog open={modelOpen} onOpenChange={setModelOpen}>
              <DialogTrigger asChild>
                <Button>新建模型</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>新建模型</DialogTitle>
                </DialogHeader>
                <div className="flex flex-col gap-3.5">
                  <div>
                    <Label className="mb-1.5 text-xs">映射供应商</Label>
                    <Select value={mProviderId} onValueChange={setMProviderId}>
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="选择供应商" />
                      </SelectTrigger>
                      <SelectContent>
                        {(providers ?? []).map((p) => (
                          <SelectItem key={p.ID} value={p.ID}>
                            {p.Name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <Label className="mb-1.5 text-xs">显示名称</Label>
                    <Input value={mName} onChange={(e) => setMName(e.target.value)} placeholder="GPT-4o" />
                  </div>
                  <div>
                    <Label className="mb-1.5 text-xs">供应商侧模型标识</Label>
                    <Input value={mIdentifier} onChange={(e) => setMIdentifier(e.target.value)} placeholder="gpt-4o" />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <Label className="mb-1.5 text-xs">输入价格 ¥/1M</Label>
                      <Input value={mInputPrice} onChange={(e) => setMInputPrice(e.target.value)} placeholder="18.0" />
                    </div>
                    <div>
                      <Label className="mb-1.5 text-xs">输出价格 ¥/1M</Label>
                      <Input value={mOutputPrice} onChange={(e) => setMOutputPrice(e.target.value)} placeholder="54.0" />
                    </div>
                  </div>
                </div>
                <DialogFooter>
                  <Button disabled={!mName || !mProviderId} onClick={() => void createModel()}>
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
                  <th className="p-3 font-semibold">模型名称</th>
                  <th className="p-3 font-semibold">映射供应商</th>
                  <th className="p-3 font-semibold">资费 / 1M</th>
                  <th className="p-3 font-semibold">状态</th>
                </tr>
              </thead>
              <tbody>
                {(models ?? []).map((m) => (
                  <tr key={m.ID} className="border-t border-border">
                    <td className="p-3">
                      <span className="flex items-center text-foreground">
                        <ProviderAvatar name={m.Name} kind={providerKind(m.ProviderID)} />
                        {m.Name}
                      </span>
                    </td>
                    <td className="p-3 text-muted-foreground">{providerName(m.ProviderID)}</td>
                    <td className="p-3 text-muted-foreground">
                      入 {formatCents(m.InputPriceCentsPer1M)} / 出 {formatCents(m.OutputPriceCentsPer1M)}
                    </td>
                    <td className="p-3">
                      <StatusPill tone={m.Status === "published" ? "ok" : "warn"}>
                        {m.Status === "published" ? "已发布" : "草稿"}
                      </StatusPill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {(models ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无模型</p>}
          </div>
        </TabsContent>

        <TabsContent value="routing" className="mt-3.5 flex flex-col gap-3.5">
          <div className="flex justify-end">
            <Dialog open={ruleOpen} onOpenChange={setRuleOpen}>
              <DialogTrigger asChild>
                <Button>新建规则</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>新建全局路由规则</DialogTitle>
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
                </div>
                <DialogFooter>
                  <Button disabled={!targetModelId} onClick={() => void createRule()}>
                    创建
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

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
              {(rules ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无全局路由规则</p>}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
