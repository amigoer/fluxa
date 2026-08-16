import { useEffect, useMemo, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Filters, PageHead, Select, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { Permission, useHasPermission } from "@/lib/auth"
import { downloadCsv } from "@/lib/csv"
import { fmt, fmtNum, formatDateTime } from "@/lib/format"
import type { CallLog, Member, Model, Provider, VirtualKey } from "@/lib/types"

// 调用日志 -- the widest list page: one row per gateway request. The top
// bar's search drops the operator here with ?q= prefilled, which is how a
// Request ID from a support ticket turns into a row.
export function CallLogsPage() {
  const [params, setParams] = useSearchParams()
  const logs = useApiQuery<CallLog[]>("/api/call-logs")
  const { data: models } = useApiQuery<Model[]>("/api/models")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const canSeeMembers = useHasPermission(Permission.OrgManageMembers)
  const { data: members } = useApiQuery<Member[]>(canSeeMembers ? "/api/members" : null)

  const [q, setQ] = useState(params.get("q") ?? "")
  const [modelId, setModelId] = useState("")
  const [providerId, setProviderId] = useState("")
  const [status, setStatus] = useState("")
  const [window, setWindow] = useState("1")

  // Keep the URL in step with the box, so the filtered view is a link
  // someone can paste into a ticket.
  useEffect(() => {
    const next = new URLSearchParams(params)
    if (q) next.set("q", q)
    else next.delete("q")
    setParams(next, { replace: true })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  const modelById = useMemo(() => new Map((models ?? []).map((m) => [m.ID, m])), [models])
  const providerById = useMemo(() => new Map((providers ?? []).map((p) => [p.ID, p])), [providers])
  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])
  const keyById = useMemo(() => new Map((keys ?? []).map((k) => [k.ID, k])), [keys])

  const rows = (logs.data ?? [])
    .filter((c) => {
      if (q) {
        const hay = `${c.RequestID}${memberById.get(c.MemberID)?.Name ?? ""}${modelById.get(c.ModelID)?.ModelIdentifier ?? ""}`
        if (!hay.toLowerCase().includes(q.toLowerCase())) return false
      }
      if (modelId && c.ModelID !== modelId) return false
      if (providerId && c.ProviderID !== providerId) return false
      if (status && c.Status !== status) return false
      if (window !== "all" && new Date(c.OccurredAt).getTime() < Date.now() - Number(window) * 86400000) return false
      return true
    })
    .sort((a, b) => new Date(b.OccurredAt).getTime() - new Date(a.OccurredAt).getTime())

  const total = rows.reduce((s, c) => s + c.CostCents, 0)

  const exportCsv = () =>
    downloadCsv(
      "fluxa-call-logs.csv",
      ["时间", "Request ID", "成员", "模型", "Key", "耗时(ms)", "输入 Token", "输出 Token", "费用(元)", "状态"],
      rows.map((c) => [
        formatDateTime(c.OccurredAt),
        c.RequestID,
        memberById.get(c.MemberID)?.Name ?? c.MemberID,
        modelById.get(c.ModelID)?.ModelIdentifier ?? c.ModelID,
        keyById.get(c.VirtualKeyID)?.Name ?? c.VirtualKeyID,
        c.LatencyMS,
        c.InputTokens,
        c.OutputTokens,
        (c.CostCents / 100).toFixed(2),
        c.Status === "success" ? "成功" : "失败",
      ]),
    )

  return (
    <div className="cn-page">
      <PageHead title="调用日志" sub="每一次网关请求的完整记录，可按 Request ID 精确定位">
        <button className="cn-btn" onClick={exportCsv}>
          <Icon name="download" size={14} />
          导出 CSV
        </button>
      </PageHead>

      <Filters
        placeholder="搜索 Request ID、成员或模型…"
        value={q}
        onValue={setQ}
        right={
          <span className="cn-count">
            {rows.length} 条 · 合计 {fmt(total)}
          </span>
        }
      >
        <Select
          label="模型"
          value={modelId}
          onValue={setModelId}
          options={[
            { value: "", label: "全部模型" },
            ...(models ?? []).map((m) => ({ value: m.ID, label: m.Name })),
          ]}
        />
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
          label="状态"
          value={status}
          onValue={setStatus}
          options={[
            { value: "", label: "全部状态" },
            { value: "success", label: "成功" },
            { value: "failed", label: "失败" },
          ]}
        />
        <Select
          label="时间范围"
          value={window}
          onValue={setWindow}
          options={[
            { value: "1", label: "今天" },
            { value: "7", label: "近 7 天" },
            { value: "30", label: "近 30 天" },
            { value: "all", label: "全部" },
          ]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>Request ID</th>
              <th>成员</th>
              <th>模型</th>
              <th>Key</th>
              <th className="cn-r">耗时</th>
              <th className="cn-r">Token</th>
              <th className="cn-r">费用</th>
              <th className="cn-r">状态</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((c) => {
              const model = modelById.get(c.ModelID)
              return (
                <tr key={c.ID}>
                  <td className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                    {formatDateTime(c.OccurredAt)}
                  </td>
                  <td>
                    <span className="cn-mono-cell cn-trunc" style={{ color: "var(--brand)", maxWidth: 130 }}>
                      {c.RequestID || "—"}
                    </span>
                  </td>
                  <td style={{ fontWeight: 560 }}>{memberById.get(c.MemberID)?.Name ?? "—"}</td>
                  <td>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 7 }}>
                      <Brand kind={providerById.get(c.ProviderID)?.Kind} size={13} />
                      {model?.ModelIdentifier ?? "—"}
                    </span>
                  </td>
                  <td style={{ color: "var(--ink-2)" }}>
                    <span className="cn-trunc" style={{ maxWidth: 130 }}>
                      {keyById.get(c.VirtualKeyID)?.Name ?? "—"}
                    </span>
                  </td>
                  <td className="cn-r cn-mono" style={{ color: c.LatencyMS > 3000 ? "var(--bad)" : "var(--ink-2)" }}>
                    {c.LatencyMS}ms
                  </td>
                  <td className="cn-r cn-mono" style={{ color: "var(--ink-2)" }}>
                    {c.Status === "failed" ? "—" : `${fmtNum(c.InputTokens)} / ${fmtNum(c.OutputTokens)}`}
                  </td>
                  <td className="cn-r cn-mono" style={{ fontWeight: 560 }}>
                    {c.CostCents ? fmt(c.CostCents) : "—"}
                  </td>
                  <td className="cn-r">
                    <Tag tone={c.Status === "success" ? "ok" : "bad"}>{c.Status === "success" ? "成功" : "失败"}</Tag>
                  </td>
                </tr>
              )
            })}
            <TableState
              colSpan={9}
              loading={logs.loading}
              empty={rows.length === 0}
              title={q ? `没有匹配 “${q}” 的请求` : "这段时间没有调用"}
              desc={q ? "换个关键词，或把时间范围放宽到全部。" : undefined}
            />
          </tbody>
        </table>
      </Card>
    </div>
  )
}
