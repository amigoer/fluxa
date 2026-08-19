import { useEffect, useMemo, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Filters, PageHead, Select, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableState, Tag } from "@/components/console/ui"
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

  const total = rows.reduce((s, c) => s + c.CostMicroCents, 0)

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
        (c.CostMicroCents / 1_000_000).toFixed(2),
        c.Status === "success" ? "成功" : "失败",
      ]),
    )

  return (
    <div className="cn-page">
      <PageHead title="调用日志" sub="每一次网关请求的完整记录，可按 Request ID 精确定位">
        <Button onClick={exportCsv}>
          <Icon name="download" size={14} />
          导出 CSV
        </Button>
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
            ...(models ?? []).map((m) => ({
              value: m.ID,
              label: m.Name,
              icon: <Brand kind={m.ProviderKind} size={14} />,
            })),
          ]}
        />
        <Select
          label="供应商"
          value={providerId}
          onValue={setProviderId}
          options={[
            { value: "", label: "全部供应商" },
            ...(providers ?? []).map((p) => ({
              value: p.ID,
              label: p.Name,
              icon: <Brand kind={p.Kind} size={14} />,
            })),
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
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>Request ID</TableHead>
              <TableHead>成员</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>Key</TableHead>
              <TableHead className="text-right">耗时</TableHead>
              <TableHead className="text-right">Token</TableHead>
              <TableHead className="text-right">费用</TableHead>
              <TableHead className="text-right">状态</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((c) => {
              const model = modelById.get(c.ModelID)
              return (
                <TableRow key={c.ID}>
                  <TableCell className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                    {formatDateTime(c.OccurredAt)}
                  </TableCell>
                  <TableCell>
                    <span className="cn-mono-cell cn-trunc" style={{ color: "var(--brand)", maxWidth: 130 }}>
                      {c.RequestID || "—"}
                    </span>
                  </TableCell>
                  <TableCell style={{ fontWeight: 560 }}>{memberById.get(c.MemberID)?.Name ?? "—"}</TableCell>
                  <TableCell>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 7 }}>
                      <Brand kind={providerById.get(c.ProviderID)?.Kind} size={13} />
                      {model?.ModelIdentifier ?? "—"}
                    </span>
                  </TableCell>
                  <TableCell style={{ color: "var(--ink-2)" }}>
                    <span className="cn-trunc" style={{ maxWidth: 130 }}>
                      {keyById.get(c.VirtualKeyID)?.Name ?? "—"}
                    </span>
                  </TableCell>
                  <TableCell className="text-right cn-mono" style={{ color: c.LatencyMS > 3000 ? "var(--bad)" : "var(--ink-2)" }}>
                    {c.LatencyMS}ms
                  </TableCell>
                  <TableCell className="text-right cn-mono" style={{ color: "var(--ink-2)" }}>
                    {c.Status === "failed" ? "—" : `${fmtNum(c.InputTokens)} / ${fmtNum(c.OutputTokens)}`}
                  </TableCell>
                  <TableCell className="text-right cn-mono" style={{ fontWeight: 560 }}>
                    {c.CostMicroCents ? fmt(c.CostMicroCents) : "—"}
                  </TableCell>
                  <TableCell className="text-right">
                    <Tag tone={c.Status === "success" ? "ok" : "bad"}>{c.Status === "success" ? "成功" : "失败"}</Tag>
                  </TableCell>
                </TableRow>
              )
            })}
            <TableState
              colSpan={9}
              loading={logs.loading}
              empty={rows.length === 0}
              title={q ? `没有匹配 “${q}” 的请求` : "这段时间没有调用"}
              desc={q ? "换个关键词，或把时间范围放宽到全部。" : undefined}
            />
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
