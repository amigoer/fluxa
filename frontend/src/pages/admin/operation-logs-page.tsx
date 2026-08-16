import { useMemo, useState } from "react"
import { Icon } from "@/components/console/icon"
import { Card, Filters, PageHead, Select, TableState } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { Permission, useHasPermission } from "@/lib/auth"
import { downloadCsv } from "@/lib/csv"
import { formatDateTime } from "@/lib/format"
import type { Member, OperationAuditLog } from "@/lib/types"

// 操作审计 -- append-only record of every administrative action. Nothing
// on this page mutates anything; that is the point.
export function OperationLogsPage() {
  const logs = useApiQuery<OperationAuditLog[]>("/api/operation-logs")
  const canSeeMembers = useHasPermission(Permission.OrgManageMembers)
  const { data: members } = useApiQuery<Member[]>(canSeeMembers ? "/api/members" : null)

  const [q, setQ] = useState("")
  const [actor, setActor] = useState("")
  const [action, setAction] = useState("")
  const [window, setWindow] = useState("30")

  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])
  const actions = useMemo(
    () => [...new Set((logs.data ?? []).map((l) => l.Action))].sort(),
    [logs.data],
  )
  const actors = useMemo(
    () => [...new Set((logs.data ?? []).map((l) => l.ActorMemberID))],
    [logs.data],
  )

  const rows = (logs.data ?? [])
    .filter((l) => {
      const who = memberById.get(l.ActorMemberID)?.Name ?? ""
      if (q && !`${who}${l.Action}${l.Detail}`.toLowerCase().includes(q.toLowerCase())) return false
      if (actor && l.ActorMemberID !== actor) return false
      if (action && l.Action !== action) return false
      if (window !== "all" && new Date(l.OccurredAt).getTime() < Date.now() - Number(window) * 86400000) return false
      return true
    })
    .sort((a, b) => new Date(b.OccurredAt).getTime() - new Date(a.OccurredAt).getTime())

  const exportCsv = () =>
    downloadCsv(
      "fluxa-operation-logs.csv",
      ["时间", "操作人", "动作", "详情"],
      rows.map((l) => [
        formatDateTime(l.OccurredAt),
        memberById.get(l.ActorMemberID)?.Name ?? l.ActorMemberID,
        l.Action,
        l.Detail,
      ]),
    )

  return (
    <div className="cn-page">
      <PageHead title="操作审计" sub="所有管理动作的不可变记录，仅追加，不可删除">
        <button className="cn-btn" onClick={exportCsv}>
          <Icon name="download" size={14} />
          导出 CSV
        </button>
      </PageHead>

      <Filters
        placeholder="搜索操作人、动作或详情…"
        value={q}
        onValue={setQ}
        right={<span className="cn-count">{rows.length} 条</span>}
      >
        <Select
          label="操作人"
          value={actor}
          onValue={setActor}
          options={[
            { value: "", label: "全部操作人" },
            ...actors.map((id) => ({ value: id, label: memberById.get(id)?.Name ?? id.slice(0, 8) })),
          ]}
        />
        <Select
          label="动作"
          value={action}
          onValue={setAction}
          options={[{ value: "", label: "全部动作" }, ...actions.map((a) => ({ value: a, label: a }))]}
        />
        <Select
          label="时间范围"
          value={window}
          onValue={setWindow}
          options={[
            { value: "7", label: "近 7 天" },
            { value: "30", label: "近 30 天" },
            { value: "90", label: "近 90 天" },
            { value: "all", label: "全部" },
          ]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>操作人</th>
              <th>动作</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((l) => {
              const who = memberById.get(l.ActorMemberID)
              return (
                <tr key={l.ID}>
                  <td className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                    {formatDateTime(l.OccurredAt)}
                  </td>
                  <td>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontWeight: 560 }}>
                      <span className="cn-av" style={{ width: 22, height: 22, fontSize: 10 }}>
                        {(who?.Name ?? "?").slice(0, 1)}
                      </span>
                      {who?.Name ?? l.ActorMemberID.slice(0, 8)}
                    </span>
                  </td>
                  <td>
                    <span className="cn-mono-cell" style={{ color: "var(--ink)" }}>
                      {l.Action}
                    </span>
                  </td>
                  <td style={{ color: "var(--ink-3)" }}>
                    <span className="cn-trunc" style={{ maxWidth: 420 }}>
                      {l.Detail || "—"}
                    </span>
                  </td>
                </tr>
              )
            })}
            <TableState
              colSpan={4}
              loading={logs.loading}
              empty={rows.length === 0}
              title="没有操作记录"
              desc="管理动作发生后会自动写入这里，无法修改或删除。"
            />
          </tbody>
        </table>
      </Card>
    </div>
  )
}
