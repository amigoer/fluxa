import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useApiQuery } from "@/hooks/use-api-query"
import { useHasPermission, Permission } from "@/lib/auth"
import { api } from "@/lib/api"
import { formatCents } from "@/lib/format"
import type { Model, QuotaRequest } from "@/lib/types"

const statusLabel = { pending: "待审批", approved: "已通过", rejected: "已驳回" } as const
const statusTone = { pending: "warn", approved: "ok", rejected: "bad" } as const

export function QuotaRequestsPage() {
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const { data: mine, refetch: refetchMine } = useApiQuery<QuotaRequest[]>("/api/quota-requests/mine")
  const isDepartmentLead = useHasPermission(Permission.OrgApproveDepartmentQuota)
  const isAnyApprover = useHasPermission(Permission.QuotaApproveAny)
  const canReview = isDepartmentLead || isAnyApprover
  const { data: pending, refetch: refetchPending } = useApiQuery<QuotaRequest[]>(
    "/api/quota-requests/pending",
    [canReview],
  )
  const modelName = (id: string | null) => (models ?? []).find((m) => m.ID === id)?.Name ?? "任意模型"

  const [modelId, setModelId] = useState("")
  const [amount, setAmount] = useState("")
  const [reason, setReason] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const submit = async () => {
    setSubmitting(true)
    try {
      await api.post("/api/quota-requests", { ModelID: modelId || null, AmountCents: Math.round(Number(amount) * 100), Reason: reason })
      setAmount("")
      setReason("")
      refetchMine()
      toast.success("申请已提交")
    } catch {
      toast.error("提交失败")
    } finally {
      setSubmitting(false)
    }
  }

  const decide = async (id: string, approve: boolean) => {
    try {
      await api.post(`/api/quota-requests/${id}/decide`, { approve })
      refetchPending()
      toast.success(approve ? "已通过" : "已驳回")
    } catch {
      toast.error("操作失败")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="配额申请" />

      <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">新建申请</p>
          <div className="flex max-w-[360px] flex-col gap-3.5">
            <div>
              <Label className="mb-1.5 text-xs">申请模型</Label>
              <Select value={modelId} onValueChange={setModelId}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="选择模型…" />
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
              <Label className="mb-1.5 text-xs">申请额度（¥）</Label>
              <Input value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0" />
            </div>
            <div>
              <Label className="mb-1.5 text-xs">用途说明</Label>
              <Textarea value={reason} onChange={(e) => setReason(e.target.value)} placeholder="简要说明用途…" />
            </div>
            <Button disabled={submitting || !amount} className="self-start" onClick={() => void submit()}>
              提交申请
            </Button>
          </div>
        </div>

        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">我的申请记录</p>
          <div className="flex flex-col">
            {(mine ?? []).map((q) => (
              <div key={q.ID} className="flex items-center justify-between border-t border-border py-2 text-xs first:border-t-0">
                <span className="text-foreground">
                  {formatCents(q.AmountCents)} · {modelName(q.ModelID)}
                </span>
                <StatusPill tone={statusTone[q.Status]}>{statusLabel[q.Status]}</StatusPill>
              </div>
            ))}
            {(mine ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">还没有申请记录</p>}
          </div>
        </div>
      </div>

      {canReview && (
        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">待我审批</p>
          <div className="flex flex-col">
            {(pending ?? []).map((q) => (
              <div key={q.ID} className="flex items-center justify-between gap-2.5 border-t border-border py-2.5 text-xs first:border-t-0">
                <span className="text-foreground">
                  {formatCents(q.AmountCents)} · {modelName(q.ModelID)}
                  {q.Reason && <span className="ml-1.5 text-muted-foreground">· {q.Reason}</span>}
                </span>
                <div className="flex flex-none gap-3">
                  <button className="font-semibold text-primary" onClick={() => void decide(q.ID, true)}>
                    通过
                  </button>
                  <button className="font-semibold text-muted-foreground" onClick={() => void decide(q.ID, false)}>
                    驳回
                  </button>
                </div>
              </div>
            ))}
            {(pending ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">没有待审批的申请</p>}
          </div>
        </div>
      )}
    </div>
  )
}
