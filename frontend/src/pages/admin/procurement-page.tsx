import { useMemo, useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { KpiCard } from "@/components/shared/kpi-card"
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
import { formatCents, formatDate } from "@/lib/format"
import type { ProcurementRecord, Provider } from "@/lib/types"

export function ProcurementPage() {
  const { data: records, refetch } = useApiQuery<ProcurementRecord[]>("/api/procurement")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const providerName = (id: string) => (providers ?? []).find((p) => p.ID === id)?.Name ?? id
  const providerKind = (id: string) => (providers ?? []).find((p) => p.ID === id)?.Kind

  const [open, setOpen] = useState(false)
  const [providerId, setProviderId] = useState("")
  const [amount, setAmount] = useState("")
  const [note, setNote] = useState("")

  const monthTotal = useMemo(() => {
    const now = new Date()
    return (records ?? [])
      .filter((r) => {
        const d = new Date(r.RecordedAt)
        return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear()
      })
      .reduce((sum, r) => sum + r.AmountCents, 0)
  }, [records])

  const record = async () => {
    try {
      await api.post("/api/procurement", { ProviderID: providerId, AmountCents: Math.round(Number(amount) * 100), Note: note })
      setOpen(false)
      setAmount("")
      setNote("")
      refetch()
      toast.success("已登记入库")
    } catch {
      toast.error("登记失败")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="入库记录" />

      <div className="grid grid-cols-2 gap-3">
        <KpiCard label="本月入库总额" value={formatCents(monthTotal)} />
        <KpiCard label="入库笔数" value={String((records ?? []).length)} />
      </div>

      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>登记入库</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>登记入库</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-3.5">
              <div>
                <Label className="mb-1.5 text-xs">供应商</Label>
                <Select value={providerId} onValueChange={setProviderId}>
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
                <Label className="mb-1.5 text-xs">金额（¥）</Label>
                <Input value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="50000" />
              </div>
              <div>
                <Label className="mb-1.5 text-xs">备注</Label>
                <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder="Q3 预算（可选）" />
              </div>
            </div>
            <DialogFooter>
              <Button disabled={!providerId || !amount} onClick={() => void record()}>
                登记
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">供应商</th>
              <th className="p-3 font-semibold">时间</th>
              <th className="p-3 font-semibold">金额</th>
              <th className="p-3 font-semibold">备注</th>
            </tr>
          </thead>
          <tbody>
            {(records ?? []).map((r) => (
              <tr key={r.ID} className="border-t border-border">
                <td className="p-3">
                  <span className="flex items-center text-foreground">
                    <ProviderAvatar name={providerName(r.ProviderID)} kind={providerKind(r.ProviderID)} />
                    {providerName(r.ProviderID)}
                  </span>
                </td>
                <td className="p-3 text-muted-foreground">{formatDate(r.RecordedAt)}</td>
                <td className="p-3 font-mono tabular-nums">{formatCents(r.AmountCents)}</td>
                <td className="p-3 text-muted-foreground">{r.Note || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(records ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无入库记录</p>}
      </div>
    </div>
  )
}
