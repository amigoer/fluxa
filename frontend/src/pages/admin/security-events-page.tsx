import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatDateTime } from "@/lib/format"
import type { SecurityEvent } from "@/lib/types"

export function SecurityEventsPage() {
  const { data: events } = useApiQuery<SecurityEvent[]>("/api/security-events")

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="安全事件" />
      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <div className="flex flex-col">
          {(events ?? []).map((e) => (
            <div key={e.ID} className="flex items-center justify-between gap-2.5 border-t border-border py-2.5 text-xs first:border-t-0">
              <span className="text-foreground">{e.Description}</span>
              <div className="flex items-center gap-2.5">
                <span className="text-muted-foreground">{formatDateTime(e.OccurredAt)}</span>
                <StatusPill tone={e.ActionTaken === "block" ? "bad" : "ok"}>{e.ActionTaken === "block" ? "拦截" : "脱敏"}</StatusPill>
              </div>
            </div>
          ))}
          {(events ?? []).length === 0 && <p className="py-2 text-xs text-muted-foreground">暂无安全事件</p>}
        </div>
      </div>
    </div>
  )
}
