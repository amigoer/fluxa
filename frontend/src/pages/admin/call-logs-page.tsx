import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { Input } from "@/components/ui/input"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatCents, formatDateTime } from "@/lib/format"
import type { CallLog } from "@/lib/types"

export function CallLogsPage() {
  const { data: logs } = useApiQuery<CallLog[]>("/api/call-logs")

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="调用日志" />

      <Input placeholder="搜索 Request ID…" className="max-w-[260px]" />

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">时间</th>
              <th className="p-3 font-semibold">状态</th>
              <th className="p-3 font-semibold">耗时</th>
              <th className="p-3 font-semibold">Token</th>
              <th className="p-3 text-right font-semibold">费用</th>
            </tr>
          </thead>
          <tbody>
            {(logs ?? []).map((l) => (
              <tr key={l.ID} className="border-t border-border">
                <td className="p-3 text-muted-foreground">{formatDateTime(l.OccurredAt)}</td>
                <td className="p-3">
                  <StatusPill tone={l.Status === "success" ? "ok" : "bad"}>{l.Status === "success" ? "成功" : "失败"}</StatusPill>
                </td>
                <td className="p-3 text-muted-foreground">{l.LatencyMS}ms</td>
                <td className="p-3 text-muted-foreground">{(l.InputTokens + l.OutputTokens).toLocaleString()}</td>
                <td className="p-3 text-right font-mono tabular-nums">{formatCents(l.CostCents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(logs ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无调用记录</p>}
      </div>
    </div>
  )
}
