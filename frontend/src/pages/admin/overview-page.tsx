import { useId, useMemo, useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Empty, Gauge, PageHead, Spark, Tag } from "@/components/console/ui"
import { ProcurementModal } from "@/components/console/procurement-modal"
import { useConsole } from "@/layouts/console-layout"
import { useApiQuery } from "@/hooks/use-api-query"
import { dailyBuckets, smoothPathScaled } from "@/lib/chart"
import { downloadCsv } from "@/lib/csv"
import { fmt, fmtNum, fmtShort, formatDate, formatDateCN, formatTime, makeMoneyFmt } from "@/lib/format"
import type {
  CallLog,
  Department,
  Member,
  Model,
  ProcurementRecord,
  Provider,
  ProviderHealth,
  QuotaRequest,
  VirtualKey,
} from "@/lib/types"

// 概览 -- the dashboard page type (DESIGN.md 6.3).
//
// The home screen has no separate to-do column: whichever KPI is in
// trouble grows a "需处理" flag and an entry point carrying its count,
// and that opens the drawer pre-filtered to that kind. The KPI answers
// "where and how bad", the drawer answers "all of them, and what to do" --
// which is what keeps the page height independent of the queue length.
//
// All three rows share one 4-column grid and span into it (1/1/1/1,
// 2/1/1, 2/2), so the vertical rules line up by construction instead of
// by three separately-tuned ratios.

const SHARE_COLORS = ["#2f5fea", "#5b8cff", "#8b5cf6", "#f59e0b", "#10b981", "#94a3b8"]

const RANGES = [
  { key: 7, label: "7 天" },
  { key: 30, label: "30 天" },
  { key: 90, label: "本季" },
]

export function OverviewPage() {
  const navigate = useNavigate()
  const { countOf, worstOf, openDrawer, reloadTodos } = useConsole()
  const [days, setDays] = useState(30)
  const [recording, setRecording] = useState(false)

  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const { data: health } = useApiQuery<ProviderHealth[]>("/api/provider-health")
  const { data: pending } = useApiQuery<QuotaRequest[]>("/api/quota-requests/pending")
  const procurement = useApiQuery<ProcurementRecord[]>("/api/procurement")
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs")
  const { data: models } = useApiQuery<Model[]>("/api/models")
  const { data: departments } = useApiQuery<Department[]>("/api/departments")
  const { data: members } = useApiQuery<Member[]>("/api/members")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")

  const rows = useMemo(() => calls ?? [], [calls])
  const now = new Date()
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1)

  const providerById = useMemo(() => new Map((providers ?? []).map((p) => [p.ID, p])), [providers])
  const modelById = useMemo(() => new Map((models ?? []).map((m) => [m.ID, m])), [models])
  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])

  const monthCalls = useMemo(
    () => rows.filter((c) => new Date(c.OccurredAt) >= monthStart),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rows],
  )
  const monthSpend = monthCalls.reduce((s, c) => s + c.CostCents, 0)

  // 上月同期: same day-of-month window, so a comparison on the 5th is
  // against the previous month's first five days rather than its whole run.
  const prevMonthCalls = useMemo(() => {
    const start = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    const end = new Date(now.getFullYear(), now.getMonth() - 1, now.getDate(), now.getHours(), now.getMinutes())
    return rows.filter((c) => {
      const d = new Date(c.OccurredAt)
      return d >= start && d <= end
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows])

  const callDelta = pct(monthCalls.length, prevMonthCalls.length)

  // 本月预算: what was actually topped up this month. Budget in this
  // product is not a number someone types -- it is the procurement ledger,
  // so the gauge reads against real money in rather than an aspiration.
  const monthProcurement = useMemo(
    () => (procurement.data ?? []).filter((r) => new Date(r.RecordedAt) >= monthStart),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [procurement.data],
  )
  const budget = monthProcurement.reduce((s, r) => s + r.AmountCents, 0)
  const usedPct = budget > 0 ? Math.min(100, (monthSpend / budget) * 100) : 0

  const trend = useMemo(
    () => dailyBuckets(rows, days, (c) => c.OccurredAt, (c) => c.CostCents),
    [rows, days],
  )
  const trendPrev = useMemo(() => {
    const shifted = rows.filter((c) => {
      const t = new Date(c.OccurredAt).getTime()
      return t < Date.now() - (days - 1) * 86400000
    })
    return dailyBuckets(
      shifted.map((c) => ({ ...c, OccurredAt: new Date(new Date(c.OccurredAt).getTime() + days * 86400000).toISOString() })),
      days,
      (c) => c.OccurredAt,
      (c) => c.CostCents,
    )
  }, [rows, days])

  const trendTotal = trend.reduce((s, d) => s + d.total, 0)
  const trendPrevTotal = trendPrev.reduce((s, d) => s + d.total, 0)
  const trendDelta = pct(trendTotal, trendPrevTotal)

  // 近 7 日均速 -> when the month's top-up runs out.
  const burn = trend.slice(-7).reduce((s, d) => s + d.total, 0) / 7
  const daysLeft = burn > 0 ? Math.floor(Math.max(0, budget - monthSpend) / burn) : null
  const exhaustAt = daysLeft === null ? null : new Date(Date.now() + daysLeft * 86400000)

  const share = useMemo(() => {
    const spend = new Map<string, number>()
    for (const c of monthCalls) spend.set(c.ProviderID, (spend.get(c.ProviderID) ?? 0) + c.CostCents)
    return [...spend.entries()]
      .map(([id, cents]) => ({ id, cents, provider: providerById.get(id) }))
      .filter((r) => r.cents > 0)
      .sort((a, b) => b.cents - a.cents)
      .slice(0, 6)
  }, [monthCalls, providerById])

  const deptRows = useMemo(() => {
    const spend = new Map<string, number>()
    const prev = new Map<string, number>()
    const add = (map: Map<string, number>, memberId: string, cents: number) => {
      const dept = memberById.get(memberId)?.DepartmentID
      if (!dept) return
      map.set(dept, (map.get(dept) ?? 0) + cents)
    }
    for (const c of monthCalls) add(spend, c.MemberID, c.CostCents)
    for (const c of prevMonthCalls) add(prev, c.MemberID, c.CostCents)

    const headcount = new Map<string, number>()
    for (const m of members ?? []) {
      if (m.DepartmentID) headcount.set(m.DepartmentID, (headcount.get(m.DepartmentID) ?? 0) + 1)
    }
    // 额度使用率 comes from the department's own keys -- that is where a
    // department's ceiling actually lives in this product.
    const pool = new Map<string, { budget: number; spent: number }>()
    for (const k of keys ?? []) {
      if (k.OwnerType !== "department" || !k.OwnerDepartmentID || k.Status !== "active") continue
      const cur = pool.get(k.OwnerDepartmentID) ?? { budget: 0, spent: 0 }
      cur.budget += k.BudgetCents
      cur.spent += k.SpentCents
      pool.set(k.OwnerDepartmentID, cur)
    }

    return (departments ?? [])
      .map((d) => {
        const p = pool.get(d.ID)
        return {
          id: d.ID,
          name: d.Name,
          members: headcount.get(d.ID) ?? 0,
          spend: spend.get(d.ID) ?? 0,
          delta: pct(spend.get(d.ID) ?? 0, prev.get(d.ID) ?? 0),
          rate: p && p.budget > 0 ? (p.spent / p.budget) * 100 : null,
        }
      })
      .sort((a, b) => b.spend - a.spend)
      .slice(0, 5)
  }, [monthCalls, prevMonthCalls, members, memberById, departments, keys])

  const recentCalls = useMemo(
    () =>
      [...rows]
        .sort((a, b) => new Date(b.OccurredAt).getTime() - new Date(a.OccurredAt).getTime())
        .slice(0, 5),
    [rows],
  )

  const healthy = (health ?? []).filter((h) => h.State === "normal").length
  const openCircuits = (health ?? []).filter((h) => h.State === "circuit_open").length
  const halfOpen = (health ?? []).filter((h) => h.State === "half_open").length
  const staleApprovals = (pending ?? []).filter(
    (q) => Date.now() - new Date(q.CreatedAt).getTime() > 3 * 86400000,
  ).length

  const exportReport = () => {
    downloadCsv(
      `fluxa-usage-${days}d.csv`,
      ["日期", "花费(元)", "调用数"],
      trend.map((d) => [
        d.day.toISOString().slice(0, 10),
        (d.total / 100).toFixed(2),
        rows.filter((c) => new Date(c.OccurredAt).toDateString() === d.day.toDateString()).length,
      ]),
    )
  }

  return (
    <div className="cn-page">
      <PageHead title="概览" sub={`${formatDateCN(monthStart.toISOString(), true)} — ${formatDateCN(now.toISOString())}`}>
        <div className="cn-seg">
          {RANGES.map((r) => (
            <button key={r.key} data-on={days === r.key} onClick={() => setDays(r.key)}>
              {r.label}
            </button>
          ))}
        </div>
        <button className="cn-btn" onClick={exportReport}>
          <Icon name="download" size={14} />
          导出报表
        </button>
        <button className="cn-btn cn-btn-pri" onClick={() => setRecording(true)}>
          <Icon name="package-plus" size={14} />
          登记入库
        </button>
      </PageHead>

      <div className="cn-kpis">
        <div className="cn-kpi" data-sev={countOf("预算") ? worstOf("预算") : undefined}>
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico">
              <Icon name="wallet" size={13} />
            </span>
            本月花费
            {countOf("预算") > 0 && <span className="cn-kpi-flag">需处理</span>}
          </div>
          <div className="cn-kpi-val">{fmt(monthSpend)}</div>
          <div className="cn-kpi-meter">
            <i style={{ width: `${usedPct}%` }} />
          </div>
          <div className="cn-kpi-foot">
            {budget > 0 ? `本月入库 ${fmtShort(budget)} · 已用 ${usedPct.toFixed(1)}%` : "本月还没有入库记录"}
          </div>
          {countOf("预算") > 0 && (
            <button className="cn-kpi-action" onClick={() => openDrawer("预算")}>
              {countOf("预算")} 个 Key 额度将耗尽 <Icon name="arrow-right" size={12} />
            </button>
          )}
        </div>

        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="ok">
              <Icon name="activity" size={13} />
            </span>
            调用量
          </div>
          <div className="cn-kpi-val">
            {fmtNum(monthCalls.length)} <small>次</small>
          </div>
          <div className="cn-kpi-foot">
            <Delta value={callDelta} />
            较上月同期
          </div>
          <Spark values={trend.slice(-14).map((d) => d.total)} color="#12805c" />
        </div>

        <div className="cn-kpi" data-sev={countOf("审批") ? worstOf("审批") : undefined}>
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="warn">
              <Icon name="clock" size={13} />
            </span>
            待审批配额
            {countOf("审批") > 0 && <span className="cn-kpi-flag">需处理</span>}
          </div>
          <div className="cn-kpi-val">
            {(pending ?? []).length} <small>件</small>
          </div>
          <div className="cn-kpi-foot">
            {staleApprovals > 0 ? (
              <span className="cn-down">{staleApprovals} 件已超 3 天未处理</span>
            ) : (
              "没有超时未处理的申请"
            )}
          </div>
          {countOf("审批") > 0 && (
            <button className="cn-kpi-action" onClick={() => openDrawer("审批")}>
              去审批 {countOf("审批")} 件 <Icon name="arrow-right" size={12} />
            </button>
          )}
        </div>

        <div className="cn-kpi" data-sev={countOf("熔断") ? worstOf("熔断") : undefined}>
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t={openCircuits > 0 ? "bad" : "ok"}>
              <Icon name="gauge" size={13} />
            </span>
            Provider 健康
            {countOf("熔断") > 0 && <span className="cn-kpi-flag">需处理</span>}
          </div>
          <div className="cn-kpi-val">
            {healthy} <small>/ {(health ?? []).length}</small>
          </div>
          <div className="cn-kpi-foot">
            {openCircuits > 0 && <Tag tone="bad">{openCircuits} 熔断</Tag>}
            {halfOpen > 0 && <Tag tone="warn">{halfOpen} 半开</Tag>}
            {openCircuits + halfOpen === 0 && "全部正常"}
          </div>
          {countOf("熔断") > 0 && (
            <button className="cn-kpi-action" onClick={() => openDrawer("熔断")}>
              去处理 {countOf("熔断")} 项 <Icon name="arrow-right" size={12} />
            </button>
          )}
        </div>
      </div>

      <div className="cn-grid">
        <TrendCard
          days={days}
          trend={trend}
          prev={trendPrev}
          total={trendTotal}
          delta={trendDelta}
          onOpenLogs={() => navigate("/admin/call-logs")}
        />

        <Card title="供应商花费构成" link="供应商" onLink={() => navigate("/admin/providers")}>
          <div className="cn-card-body">
            {share.length === 0 ? (
              <Empty icon="server" title="本月还没有调用" desc="接入供应商并发起第一次调用后，这里会显示花费构成。" />
            ) : (
              <ShareBody share={share} total={monthSpend} />
            )}
          </div>
        </Card>

        <Card title="本月预算" link="入库记录" onLink={() => navigate("/admin/procurement")}>
          <div className="cn-card-body">
            <div className="cn-gauge-wrap">
              <Gauge pct={usedPct} size={68} />
              <div style={{ minWidth: 0 }}>
                <div className="cn-budget-num">{fmtShort(Math.max(0, budget - monthSpend))}</div>
                <div className="cn-budget-sub">剩余可用 · 共 {fmtShort(budget)}</div>
              </div>
            </div>
            <div className="cn-budget-sub" style={{ marginTop: 10 }}>
              {exhaustAt && budget > 0 ? (
                <>
                  预计 <b style={{ color: "var(--ink)" }}>{formatDateCN(exhaustAt.toISOString())}</b> 触顶 · 按近 7 日均速
                </>
              ) : (
                "近 7 日没有花费，无法估算触顶时间"
              )}
            </div>
            <div className="cn-budget-recent">
              <div className="cn-budget-recent-label">
                近期入库
                <Link to="/admin/procurement">全部</Link>
              </div>
              {(procurement.data ?? []).slice(0, 3).map((r) => (
                <div key={r.ID} className="cn-mini-row">
                  <span className="cn-mini-date">{formatDate(r.RecordedAt)}</span>
                  <span className="cn-mini-name">
                    <Brand kind={providerById.get(r.ProviderID)?.Kind} size={13} />
                    {providerById.get(r.ProviderID)?.Name ?? "未知供应商"}
                  </span>
                  <span className="cn-mini-amt">{fmtShort(r.AmountCents)}</span>
                </div>
              ))}
              {(procurement.data ?? []).length === 0 && (
                <div className="cn-budget-sub" style={{ marginTop: 8 }}>
                  还没有入库记录。
                </div>
              )}
            </div>
          </div>
        </Card>
      </div>

      <div className="cn-grid">
        <Card
          className="cn-span-2 cn-preview"
          title="部门花费"
          note="本月 · 含部门 Key 额度使用率"
          link="成员与部门"
          onLink={() => navigate("/admin/members")}
        >
          <table className="cn-table">
            <thead>
              <tr>
                <th>部门</th>
                <th className="cn-col-optional">人数</th>
                <th>额度使用率</th>
                <th className="cn-r">环比</th>
                <th className="cn-r">花费</th>
              </tr>
            </thead>
            <tbody>
              {deptRows.map((d) => (
                <tr key={d.id}>
                  <td style={{ fontWeight: 560 }}>{d.name}</td>
                  <td className="cn-mono cn-col-optional" style={{ color: "var(--ink-2)" }}>
                    {d.members}
                  </td>
                  <td>
                    {d.rate === null ? (
                      <span style={{ color: "var(--ink-3)" }}>未设额度</span>
                    ) : (
                      <div className="cn-usage">
                        <span className="cn-usage-track">
                          <i data-over={d.rate > 90} style={{ width: `${Math.min(100, d.rate)}%` }} />
                        </span>
                        <span className="cn-mono" style={{ color: d.rate > 90 ? "var(--warn)" : "var(--ink-3)" }}>
                          {d.rate.toFixed(0)}%
                        </span>
                      </div>
                    )}
                  </td>
                  <td className="cn-r">
                    <Delta value={d.delta} invert />
                  </td>
                  <td className="cn-r cn-mono" style={{ fontSize: 12.5, fontWeight: 560 }}>
                    {fmt(d.spend)}
                  </td>
                </tr>
              ))}
              {deptRows.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ padding: 0 }}>
                    <Empty icon="users" title="还没有部门" desc="在「成员与部门」里建好部门后，这里按部门汇总花费。" />
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </Card>

        <Card
          className="cn-span-2 cn-preview"
          title="实时调用"
          note="最近 5 条"
          link="调用日志"
          onLink={() => navigate("/admin/call-logs")}
        >
          <table className="cn-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>模型</th>
                <th className="cn-col-optional">成员</th>
                <th className="cn-r">耗时</th>
                <th className="cn-r">费用</th>
                <th className="cn-r">状态</th>
              </tr>
            </thead>
            <tbody>
              {recentCalls.map((c) => {
                const model = modelById.get(c.ModelID)
                return (
                  <tr key={c.ID}>
                    <td className="cn-mono" style={{ color: "var(--ink-3)" }}>
                      {formatTime(c.OccurredAt)}
                    </td>
                    <td>
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 7 }}>
                        <Brand kind={providerById.get(c.ProviderID)?.Kind} size={13} />
                        <span style={{ fontWeight: 540 }}>{model?.Name ?? model?.ModelIdentifier ?? "—"}</span>
                      </span>
                    </td>
                    <td className="cn-col-optional" style={{ color: "var(--ink-2)" }}>
                      {memberById.get(c.MemberID)?.Name ?? "—"}
                    </td>
                    <td className="cn-r cn-mono" style={{ color: c.LatencyMS > 3000 ? "var(--bad)" : "var(--ink-2)" }}>
                      {c.LatencyMS}ms
                    </td>
                    <td className="cn-r cn-mono">{c.CostCents ? fmt(c.CostCents) : "—"}</td>
                    <td className="cn-r">
                      <Tag tone={c.Status === "success" ? "ok" : "bad"}>{c.Status === "success" ? "成功" : "失败"}</Tag>
                    </td>
                  </tr>
                )
              })}
              {recentCalls.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ padding: 0 }}>
                    <Empty icon="activity" title="还没有调用" desc="员工用虚拟 Key 发起第一次请求后，这里会实时滚动。" />
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </Card>
      </div>

      <ProcurementModal
        open={recording}
        providers={providers ?? []}
        onClose={() => setRecording(false)}
        onDone={() => {
          procurement.refetch()
          reloadTodos()
        }}
      />
    </div>
  )
}

// ---- pieces -----------------------------------------------------------

function TrendCard({
  days,
  trend,
  prev,
  total,
  delta,
  onOpenLogs,
}: {
  days: number
  trend: { day: Date; total: number }[]
  prev: { day: Date; total: number }[]
  total: number
  delta: number | null
  onOpenLogs: () => void
}) {
  const id = useId()
  const W = 620
  const H = 172
  const values = trend.map((d) => d.total)
  const prevValues = prev.map((d) => d.total)
  const max = Math.max(1, ...values, ...prevValues)
  const ticks = [max, max * 0.75, max * 0.5, max * 0.25, 0]
  const marks = [0, Math.floor(days / 4), Math.floor(days / 2), Math.floor((days * 3) / 4), days - 1]

  return (
    <Card
      className="cn-span-2"
      title="用量趋势"
      note={`近 ${days} 天 · 按日`}
      link="调用日志"
      onLink={onOpenLogs}
      flush={false}
    >
      <div className="cn-chart-top">
        <span className="cn-chart-big">{fmt(total)}</span>
        <Delta value={delta} size={12} />
        <div className="cn-legend">
          <span>
            <i />
            本期
          </span>
          <span>
            <i className="prev" />
            上一周期
          </span>
        </div>
      </div>
      <div className="cn-plot">
        <div className="cn-yaxis">
          {ticks.map((t, i) => (
            <span key={i}>{fmtShort(t)}</span>
          ))}
        </div>
        <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} preserveAspectRatio="none">
          <defs>
            <linearGradient id={`cn${id}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#2f5fea" stopOpacity=".22" />
              <stop offset="100%" stopColor="#2f5fea" stopOpacity=".01" />
            </linearGradient>
          </defs>
          {[0, 0.25, 0.5, 0.75, 1].map((f) => (
            <line
              key={f}
              x1="0"
              x2={W}
              y1={Math.max(0.5, H * f - 0.5)}
              y2={Math.max(0.5, H * f - 0.5)}
              stroke="#e9edf4"
              strokeWidth="1"
            />
          ))}
          <path
            d={smoothPathScaled(prevValues, max, W, H, 8)}
            fill="none"
            stroke="#c3cbd8"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <path d={`${smoothPathScaled(values, max, W, H, 8)} L${W} ${H} L0 ${H} Z`} fill={`url(#cn${id})`} />
          <path d={smoothPathScaled(values, max, W, H, 8)} fill="none" stroke="#2f5fea" strokeWidth="2" />
        </svg>
        <div className="cn-xaxis">
          {marks.map((i, n) => (
            <span key={i}>{n === marks.length - 1 ? "今天" : formatDate(trend[i]?.day.toISOString() ?? "")}</span>
          ))}
        </div>
      </div>
    </Card>
  )
}

function ShareBody({
  share,
  total,
}: {
  share: { id: string; cents: number; provider?: Provider }[]
  total: number
}) {
  const money = makeMoneyFmt(Math.max(...share.map((s) => s.cents)))
  return (
    <>
      <div className="cn-stack">
        {share.map((s, i) => (
          <i key={s.id} style={{ width: `${(s.cents / (total || 1)) * 100}%`, background: SHARE_COLORS[i] }} />
        ))}
      </div>
      <div className="cn-share">
        {share.map((s, i) => (
          <div key={s.id} className="cn-share-row">
            <span className="cn-swatch" style={{ background: SHARE_COLORS[i] }} />
            <span className="cn-share-name">
              <Brand kind={s.provider?.Kind} size={13} />
              <span>{s.provider?.Name ?? "未知供应商"}</span>
            </span>
            <span className="cn-share-pct">{((s.cents / (total || 1)) * 100).toFixed(1)}%</span>
            <span className="cn-share-val">{money(s.cents)}</span>
          </div>
        ))}
      </div>
    </>
  )
}

// A change of +12% is good news for call volume and bad news for spend,
// so the caller says which way to colour it.
function Delta({ value, invert, size = 11.5 }: { value: number | null; invert?: boolean; size?: number }) {
  if (value === null) return <span style={{ fontSize: size, color: "var(--ink-3)" }}>—</span>
  const up = value >= 0
  const bad = invert ? up : !up
  return (
    <span className={bad ? "cn-down" : "cn-up"} style={{ fontSize: size }}>
      <Icon name={up ? "trending-up" : "trending-down"} size={12} />
      {Math.abs(value).toFixed(1)}%
    </span>
  )
}

function pct(now: number, before: number): number | null {
  if (!before) return null
  return ((now - before) / before) * 100
}

