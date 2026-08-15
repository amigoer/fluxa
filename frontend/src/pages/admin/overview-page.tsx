import { useMemo } from "react"
import { PageHeader } from "@/components/shared/page-header"
import { KpiCard } from "@/components/shared/kpi-card"
import { StatusPill } from "@/components/shared/status-pill"
import { ProviderAvatar } from "@/components/shared/provider-avatar"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatCents, formatDate } from "@/lib/format"
import type { CallLog, Provider, ProviderHealth, ProcurementRecord, QuotaRequest, SecurityEvent } from "@/lib/types"

const healthTone = { normal: "ok", circuit_open: "bad", half_open: "warn" } as const
const healthLabel = { normal: "正常", circuit_open: "熔断", half_open: "半开" } as const

export function OverviewPage() {
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const { data: health } = useApiQuery<ProviderHealth[]>("/api/provider-health")
  const { data: pending } = useApiQuery<QuotaRequest[]>("/api/quota-requests/pending")
  const { data: procurement } = useApiQuery<ProcurementRecord[]>("/api/procurement")
  const { data: events } = useApiQuery<SecurityEvent[]>("/api/security-events")
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs")

  const monthSpendCents = useMemo(() => {
    if (!calls) return 0
    const now = new Date()
    return calls
      .filter((c) => {
        const d = new Date(c.OccurredAt)
        return d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear()
      })
      .reduce((sum, c) => sum + c.CostCents, 0)
  }, [calls])

  const dailyBars = useMemo(() => {
    const days: { label: string; cents: number }[] = []
    for (let i = 13; i >= 0; i--) {
      const d = new Date()
      d.setDate(d.getDate() - i)
      const dayCalls = (calls ?? []).filter((c) => {
        const cd = new Date(c.OccurredAt)
        return cd.toDateString() === d.toDateString()
      })
      days.push({ label: formatDate(d.toISOString()), cents: dayCalls.reduce((s, c) => s + c.CostCents, 0) })
    }
    return days
  }, [calls])
  const maxCents = Math.max(1, ...dailyBars.map((d) => d.cents))

  const healthByProvider = new Map((health ?? []).map((h) => [h.ProviderID, h]))
  const normalCount = (health ?? []).filter((h) => h.State === "normal").length

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="概览" />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <KpiCard label="本月花费" value={formatCents(monthSpendCents)} />
        <KpiCard label="活跃供应商" value={String((providers ?? []).filter((p) => p.Status === "active").length)} />
        <KpiCard label="待审批配额申请" value={String((pending ?? []).length)} />
        <KpiCard
          label="Provider 健康"
          value={`${normalCount}/${(health ?? []).length}`}
          delta={(health ?? []).length - normalCount > 0 ? `${(health ?? []).length - normalCount} 异常` : undefined}
          deltaTone="down"
        />
      </div>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1.7fr_1fr]">
        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">用量趋势 · 近 14 天</p>
          <div className="mb-2 flex h-[100px] items-end gap-1">
            {dailyBars.map((d, i) => (
              <div
                key={i}
                className="flex-1 rounded-t-sm bg-primary"
                style={{ height: `${Math.max(3, (d.cents / maxCents) * 100)}%`, opacity: i === dailyBars.length - 1 ? 1 : 0.38 }}
                title={`${d.label} ${formatCents(d.cents)}`}
              />
            ))}
          </div>
          <div className="flex justify-between text-[10.5px] text-muted-foreground">
            <span>14 天前</span>
            <span>今天</span>
          </div>
        </div>

        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">Provider 健康状态</p>
          <div className="flex flex-col">
            {(providers ?? []).map((p) => {
              const h = healthByProvider.get(p.ID)
              const state = h?.State ?? "normal"
              return (
                <div key={p.ID} className="flex items-center justify-between border-t border-border py-2 text-xs first:border-t-0">
                  <span className="flex items-center text-foreground">
                    <ProviderAvatar name={p.Name} kind={p.Kind} />
                    {p.Name}
                  </span>
                  <StatusPill tone={healthTone[state]}>{healthLabel[state]}</StatusPill>
                </div>
              )
            })}
            {(providers ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无供应商</p>}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">最近入库记录</p>
          <table className="w-full text-[11.5px]">
            <thead>
              <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
                <th className="pb-2 font-semibold">时间</th>
                <th className="pb-2 font-semibold">金额</th>
              </tr>
            </thead>
            <tbody>
              {(procurement ?? []).slice(0, 5).map((r) => (
                <tr key={r.ID} className="border-t border-border">
                  <td className="py-2 text-muted-foreground">{formatDate(r.RecordedAt)}</td>
                  <td className="py-2 text-right font-mono tabular-nums">{formatCents(r.AmountCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {(procurement ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无记录</p>}
        </div>

        <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">最近安全事件</p>
          <div className="flex flex-col">
            {(events ?? []).slice(0, 5).map((e) => (
              <div key={e.ID} className="flex items-center justify-between border-t border-border py-2 text-xs first:border-t-0">
                <span className="text-foreground">{e.Description}</span>
                <StatusPill tone={e.ActionTaken === "block" ? "bad" : "ok"}>
                  {e.ActionTaken === "block" ? "拦截" : "脱敏"}
                </StatusPill>
              </div>
            ))}
            {(events ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无事件</p>}
          </div>
        </div>
      </div>
    </div>
  )
}
