import { useMemo, useState } from "react"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Filters, PageHead, Select, TableState } from "@/components/console/ui"
import { ProcurementModal } from "@/components/console/procurement-modal"
import { useApiQuery } from "@/hooks/use-api-query"
import { Permission, useHasPermission } from "@/lib/auth"
import { downloadCsv } from "@/lib/csv"
import { fmt, formatDay } from "@/lib/format"
import type { Member, ProcurementRecord, Provider } from "@/lib/types"

// 入库记录 -- the ledger the budget gauge on the overview reads from.
export function ProcurementPage() {
  const records = useApiQuery<ProcurementRecord[]>("/api/procurement")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const canRecord = useHasPermission(Permission.ProviderRecordProcurement)
  const canSeeMembers = useHasPermission(Permission.OrgManageMembers)
  const { data: members } = useApiQuery<Member[]>(canSeeMembers ? "/api/members" : null)

  const [q, setQ] = useState("")
  const [providerId, setProviderId] = useState("")
  const [window, setWindow] = useState("90")
  const [recording, setRecording] = useState(false)

  const providerById = useMemo(() => new Map((providers ?? []).map((p) => [p.ID, p])), [providers])
  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])

  const rows = (records.data ?? []).filter((r) => {
    const name = providerById.get(r.ProviderID)?.Name ?? ""
    if (q && !`${name}${r.Note}`.toLowerCase().includes(q.toLowerCase())) return false
    if (providerId && r.ProviderID !== providerId) return false
    if (window !== "all") {
      const cutoff = Date.now() - Number(window) * 86400000
      if (new Date(r.RecordedAt).getTime() < cutoff) return false
    }
    return true
  })
  const total = rows.reduce((s, r) => s + r.AmountCents, 0)

  const exportCsv = () =>
    downloadCsv(
      "fluxa-procurement.csv",
      ["入库时间", "供应商", "备注", "登记人", "金额(元)"],
      rows.map((r) => [
        formatDay(r.RecordedAt),
        providerById.get(r.ProviderID)?.Name ?? r.ProviderID,
        r.Note,
        memberById.get(r.RecordedByMemberID)?.Name ?? r.RecordedByMemberID,
        (r.AmountCents / 100).toFixed(2),
      ]),
    )

  return (
    <div className="cn-page">
      <PageHead title="入库记录" sub="采购充值流水，决定各供应商的可用余额">
        <button className="cn-btn" onClick={exportCsv}>
          <Icon name="download" size={14} />
          导出
        </button>
        {canRecord && (
          <button className="cn-btn cn-btn-pri" onClick={() => setRecording(true)}>
            <Icon name="package-plus" size={14} />
            登记入库
          </button>
        )}
      </PageHead>

      <Filters
        placeholder="搜索供应商或备注…"
        value={q}
        onValue={setQ}
        right={
          <span className="cn-count">
            {rows.length} 笔 · 合计 {fmt(total)}
          </span>
        }
      >
        <Select
          label="供应商"
          value={providerId}
          onValue={setProviderId}
          options={[
            { value: "", label: "全部供应商" },
            ...(providers ?? []).map((p) => ({ value: p.ID, label: p.Name })),
          ]}
        />
        <Select
          label="时间范围"
          value={window}
          onValue={setWindow}
          options={[
            { value: "30", label: "近 30 天" },
            { value: "90", label: "近 90 天" },
            { value: "365", label: "近一年" },
            { value: "all", label: "全部" },
          ]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>入库时间</th>
              <th>供应商</th>
              <th>备注</th>
              <th>登记人</th>
              <th className="cn-r">金额</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const p = providerById.get(r.ProviderID)
              return (
                <tr key={r.ID}>
                  <td className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                    {formatDay(r.RecordedAt)}
                  </td>
                  <td>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontWeight: 560 }}>
                      <Brand kind={p?.Kind} size={14} />
                      {p?.Name ?? "未知供应商"}
                    </span>
                  </td>
                  <td style={{ color: "var(--ink-3)" }}>{r.Note || "—"}</td>
                  <td style={{ color: "var(--ink-2)" }}>
                    {memberById.get(r.RecordedByMemberID)?.Name ?? "—"}
                  </td>
                  <td className="cn-r cn-mono" style={{ fontSize: 12.5, fontWeight: 600 }}>
                    {fmt(r.AmountCents)}
                  </td>
                </tr>
              )
            })}
            <TableState
              colSpan={5}
              loading={records.loading}
              empty={rows.length === 0}
              title="还没有入库记录"
              desc="登记一笔采购充值后，概览的预算表盘才有基准。"
            />
          </tbody>
        </table>
      </Card>

      <ProcurementModal
        open={recording}
        providers={providers ?? []}
        onClose={() => setRecording(false)}
        onDone={() => records.refetch()}
      />
    </div>
  )
}
