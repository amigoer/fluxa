import { useMemo, useState } from "react"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Filters, PageHead, Select, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { Permission, useHasPermission } from "@/lib/auth"
import { downloadCsv } from "@/lib/csv"
import { formatDateTime } from "@/lib/format"
import type { CallLog, DLPRule, Member, SecurityEvent, VirtualKey } from "@/lib/types"

// 安全事件 -- list page with a KPI band on top. Every row is a DLP rule
// firing; the band above answers "how much of this is happening at all",
// which is the question that decides whether a rule needs tuning.
export function SecurityEventsPage() {
  const events = useApiQuery<SecurityEvent[]>("/api/security-events")
  const canSeeRules = useHasPermission(Permission.SecurityManageDLPRules)
  const canSeeMembers = useHasPermission(Permission.OrgManageMembers)
  const canSeeLogs = useHasPermission(Permission.AuditViewCallLogs)
  const { data: rules } = useApiQuery<DLPRule[]>(canSeeRules ? "/api/dlp-rules" : null)
  const { data: members } = useApiQuery<Member[]>(canSeeMembers ? "/api/members" : null)
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const { data: calls } = useApiQuery<CallLog[]>(canSeeLogs ? "/api/call-logs" : null)

  const [q, setQ] = useState("")
  const [action, setAction] = useState("")
  const [ruleId, setRuleId] = useState("")
  const [window, setWindow] = useState("7")

  const ruleById = useMemo(() => new Map((rules ?? []).map((r) => [r.ID, r])), [rules])
  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])
  const keyById = useMemo(() => new Map((keys ?? []).map((k) => [k.ID, k])), [keys])

  const all = events.data ?? []
  const todayStart = new Date()
  todayStart.setHours(0, 0, 0, 0)
  const today = all.filter((e) => new Date(e.OccurredAt) >= todayStart)
  const blockedToday = today.filter((e) => e.ActionTaken === "block").length
  const involved = new Set(today.map((e) => e.MemberID).filter(Boolean)).size
  const callsToday = (calls ?? []).filter((c) => new Date(c.OccurredAt) >= todayStart).length
  const hitRate = callsToday > 0 ? (today.length / callsToday) * 100 : null

  const rows = all.filter((e) => {
    const who = memberById.get(e.MemberID ?? "")?.Name ?? ""
    const rule = ruleById.get(e.RuleID ?? "")?.Name ?? ""
    if (q && !`${who}${rule}${e.Description}`.toLowerCase().includes(q.toLowerCase())) return false
    if (action && e.ActionTaken !== action) return false
    if (ruleId && e.RuleID !== ruleId) return false
    if (window !== "all" && new Date(e.OccurredAt).getTime() < Date.now() - Number(window) * 86400000) return false
    return true
  })

  const exportCsv = () =>
    downloadCsv(
      "fluxa-security-events.csv",
      ["时间", "成员", "命中规则", "使用的 Key", "动作", "说明"],
      rows.map((e) => [
        formatDateTime(e.OccurredAt),
        memberById.get(e.MemberID ?? "")?.Name ?? "—",
        ruleById.get(e.RuleID ?? "")?.Name ?? "—",
        keyById.get(e.VirtualKeyID ?? "")?.Name ?? "—",
        e.ActionTaken === "block" ? "拦截" : "脱敏",
        e.Description,
      ]),
    )

  return (
    <div className="cn-page">
      <PageHead title="安全事件" sub="DLP 规则命中记录">
        <Button onClick={exportCsv}>
          <Icon name="download" size={14} />
          导出
        </Button>
      </PageHead>

      <div className="cn-kpis">
        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="bad">
              <Icon name="shield-alert" size={13} />
            </span>
            今日拦截
          </div>
          <div className="cn-kpi-val">
            {blockedToday} <small>次</small>
          </div>
          <div className="cn-kpi-foot">请求被直接终止，未发往上游</div>
        </div>
        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="warn">
              <Icon name="shield-check" size={13} />
            </span>
            今日脱敏
          </div>
          <div className="cn-kpi-val">
            {today.length - blockedToday} <small>次</small>
          </div>
          <div className="cn-kpi-foot">替换命中内容后继续转发</div>
        </div>
        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico">
              <Icon name="users" size={13} />
            </span>
            涉及成员
          </div>
          <div className="cn-kpi-val">
            {involved} <small>人</small>
          </div>
          <div className="cn-kpi-foot">今日至少命中一次规则</div>
        </div>
        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="ok">
              <Icon name="activity" size={13} />
            </span>
            命中率
          </div>
          <div className="cn-kpi-val">
            {hitRate === null ? "—" : hitRate.toFixed(2)} <small>%</small>
          </div>
          <div className="cn-kpi-foot">占今日全部请求</div>
        </div>
      </div>

      <Filters
        placeholder="搜索成员、规则或说明…"
        value={q}
        onValue={setQ}
        right={<span className="cn-count">{rows.length} 条</span>}
      >
        <Select
          label="动作"
          value={action}
          onValue={setAction}
          options={[
            { value: "", label: "全部动作" },
            { value: "block", label: "拦截" },
            { value: "mask", label: "脱敏" },
          ]}
        />
        <Select
          label="规则"
          value={ruleId}
          onValue={setRuleId}
          options={[{ value: "", label: "全部规则" }, ...(rules ?? []).map((r) => ({ value: r.ID, label: r.Name }))]}
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
              <TableHead>成员</TableHead>
              <TableHead>命中规则</TableHead>
              <TableHead>使用的 Key</TableHead>
              <TableHead>说明</TableHead>
              <TableHead className="text-right">动作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((e) => (
              <TableRow key={e.ID}>
                <TableCell className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                  {formatDateTime(e.OccurredAt)}
                </TableCell>
                <TableCell style={{ fontWeight: 560 }}>{memberById.get(e.MemberID ?? "")?.Name ?? "—"}</TableCell>
                <TableCell>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 7 }}>
                    <span
                      className="cn-todo-ico"
                      data-t={e.ActionTaken === "block" ? "bad" : "warn"}
                      style={{ width: 22, height: 22, borderRadius: 6 }}
                    >
                      <Icon name={e.ActionTaken === "block" ? "shield-alert" : "shield-check"} size={12} />
                    </span>
                    {ruleById.get(e.RuleID ?? "")?.Name ?? "已删除的规则"}
                  </span>
                </TableCell>
                <TableCell style={{ color: "var(--ink-2)" }}>
                  <span className="cn-trunc" style={{ maxWidth: 150 }}>
                    {keyById.get(e.VirtualKeyID ?? "")?.Name ?? "—"}
                  </span>
                </TableCell>
                <TableCell style={{ color: "var(--ink-3)" }}>
                  <span className="cn-trunc">{e.Description}</span>
                </TableCell>
                <TableCell className="text-right">
                  <Tag tone={e.ActionTaken === "block" ? "bad" : "warn"}>
                    {e.ActionTaken === "block" ? "拦截" : "脱敏"}
                  </Tag>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={6}
              loading={events.loading}
              empty={rows.length === 0}
              title="没有安全事件"
              desc="这段时间里没有请求命中任何 DLP 规则。"
            />
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
