import { useMemo } from "react"
import { PageHeader } from "@/components/shared/page-header"
import { KpiCard } from "@/components/shared/kpi-card"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatCents } from "@/lib/format"
import type { CallLog, VirtualKey } from "@/lib/types"

export function UsagePage() {
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs/mine")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")

  const monthCalls = useMemo(() => {
    const now = new Date()
    return (calls ?? []).filter((c) => {
      const d = new Date(c.OccurredAt)
      return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear()
    })
  }, [calls])

  const monthSpendCents = monthCalls.reduce((s, c) => s + c.CostCents, 0)
  const remainingCents = (keys ?? []).reduce((s, k) => s + Math.max(0, k.BudgetCents - k.SpentCents), 0)

  const dailyBars = useMemo(() => {
    const days: number[] = []
    for (let i = 13; i >= 0; i--) {
      const d = new Date()
      d.setDate(d.getDate() - i)
      const dayCents = (calls ?? [])
        .filter((c) => new Date(c.OccurredAt).toDateString() === d.toDateString())
        .reduce((s, c) => s + c.CostCents, 0)
      days.push(dayCents)
    }
    return days
  }, [calls])
  const maxCents = Math.max(1, ...dailyBars)

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="我的用量" />

      <div className="grid grid-cols-3 gap-3">
        <KpiCard label="本月消费" value={formatCents(monthSpendCents)} />
        <KpiCard label="剩余配额" value={formatCents(remainingCents)} />
        <KpiCard label="本月调用次数" value={String(monthCalls.length)} />
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">用量趋势 · 近 14 天</p>
        <div className="flex h-[100px] items-end gap-1">
          {dailyBars.map((cents, i) => (
            <div
              key={i}
              className="flex-1 rounded-t-sm bg-primary"
              style={{ height: `${Math.max(3, (cents / maxCents) * 100)}%`, opacity: i === dailyBars.length - 1 ? 1 : 0.38 }}
            />
          ))}
        </div>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">最近调用</p>
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="pb-2 font-semibold">时间</th>
              <th className="pb-2 font-semibold">Token</th>
              <th className="pb-2 text-right font-semibold">费用</th>
            </tr>
          </thead>
          <tbody>
            {(calls ?? []).slice(0, 10).map((c) => (
              <tr key={c.ID} className="border-t border-border">
                <td className="py-2 text-muted-foreground">{new Date(c.OccurredAt).toLocaleTimeString("zh-CN")}</td>
                <td className="py-2 text-muted-foreground">{(c.InputTokens + c.OutputTokens).toLocaleString()}</td>
                <td className="py-2 text-right font-mono tabular-nums">{formatCents(c.CostCents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(calls ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无调用记录</p>}
      </div>
    </div>
  )
}
