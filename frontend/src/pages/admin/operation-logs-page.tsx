import { PageHeader } from "@/components/shared/page-header"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatDateTime } from "@/lib/format"
import type { OperationAuditLog } from "@/lib/types"

export function OperationLogsPage() {
  const { data: logs } = useApiQuery<OperationAuditLog[]>("/api/operation-logs")

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="操作审计" />

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">时间</th>
              <th className="p-3 font-semibold">类型</th>
              <th className="p-3 font-semibold">详情</th>
            </tr>
          </thead>
          <tbody>
            {(logs ?? []).map((l) => (
              <tr key={l.ID} className="border-t border-border">
                <td className="p-3 text-muted-foreground">{formatDateTime(l.OccurredAt)}</td>
                <td className="p-3 text-muted-foreground">{l.Action}</td>
                <td className="p-3 text-muted-foreground">{l.Detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(logs ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无操作记录</p>}
      </div>
    </div>
  )
}
