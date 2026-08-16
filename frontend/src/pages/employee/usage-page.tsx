import { useId, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Empty, PageHead, Spark } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { useAuth } from "@/lib/auth"
import { dailyBuckets, smoothPathScaled } from "@/lib/chart"
import { fmt, fmtNum, fmtShort, formatDate, formatDateCN, makeMoneyFmt } from "@/lib/format"
import type { CallLog, Model, QuotaRequest, VirtualKey } from "@/lib/types"

// 我的用量 -- the employee's dashboard. Same components and tokens as the
// admin console, only narrower: an employee sees their own spend and
// nobody else's, and has no management actions anywhere on the page.
export function UsagePage() {
  const navigate = useNavigate()
  const { member, departmentName } = useAuth()
  const [days, setDays] = useState(30)

  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs/mine")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const { data: requests } = useApiQuery<QuotaRequest[]>("/api/quota-requests/mine")

  const rows = useMemo(() => calls ?? [], [calls])
  const monthStart = new Date()
  monthStart.setDate(1)
  monthStart.setHours(0, 0, 0, 0)

  const modelById = useMemo(() => new Map((models ?? []).map((m) => [m.ID, m])), [models])
  // /api/virtual-keys widens to the whole org for anyone holding
  // org.manage_keys, and an admin looking at 我的用量 still means *their*
  // quota -- so scope it to keys issued to this member.
  const myKeys = (keys ?? []).filter(
    (k) => k.Status === "active" && k.OwnerType === "member" && k.OwnerMemberID === member?.ID,
  )
  const quota = myKeys.reduce((s, k) => s + k.BudgetCents, 0)
  const used = myKeys.reduce((s, k) => s + k.SpentCents, 0)
  const rate = quota > 0 ? (used / quota) * 100 : 0

  const monthCalls = rows.filter((c) => new Date(c.OccurredAt) >= monthStart)
  const monthSpend = monthCalls.reduce((s, c) => s + c.CostCents, 0)
  const avgLatency = monthCalls.length
    ? Math.round(monthCalls.reduce((s, c) => s + c.LatencyMS, 0) / monthCalls.length)
    : 0
  const p95 = useMemo(() => {
    const sorted = monthCalls.map((c) => c.LatencyMS).sort((a, b) => a - b)
    if (sorted.length === 0) return 0
    return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))]
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows])

  const trend = useMemo(
    () => dailyBuckets(rows, days, (c) => c.OccurredAt, (c) => c.CostCents),
    [rows, days],
  )

  const byModel = useMemo(() => {
    const acc = new Map<string, { spend: number; calls: number }>()
    for (const c of monthCalls) {
      const cur = acc.get(c.ModelID) ?? { spend: 0, calls: 0 }
      cur.spend += c.CostCents
      cur.calls += 1
      acc.set(c.ModelID, cur)
    }
    return [...acc.entries()]
      .map(([id, v]) => ({ id, ...v, model: modelById.get(id) }))
      .sort((a, b) => b.spend - a.spend)
      .slice(0, 6)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, modelById])

  const pending = (requests ?? []).filter((r) => r.Status === "pending")
  const money = makeMoneyFmt(Math.max(1, ...byModel.map((m) => m.spend)))

  return (
    <div className="cn-page">
      <PageHead
        title="我的用量"
        sub={[departmentName, member?.Name, `本月 ${formatDateCN(monthStart.toISOString())} 至今`].filter(Boolean).join(" · ")}
      >
        <div className="cn-seg">
          <button data-on={days === 7} onClick={() => setDays(7)}>
            7 天
          </button>
          <button data-on={days === 30} onClick={() => setDays(30)}>
            本月
          </button>
        </div>
      </PageHead>

      <div className="cn-kpis">
        <div className="cn-kpi" data-sev={rate > 75 ? "warn" : undefined}>
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico">
              <Icon name="wallet" size={13} />
            </span>
            本月已用
            {rate > 75 && <span className="cn-kpi-flag">额度偏紧</span>}
          </div>
          <div className="cn-kpi-val">{fmt(monthSpend)}</div>
          <div className="cn-kpi-meter">
            <i style={{ width: `${Math.min(100, rate)}%`, background: rate > 75 ? "var(--warn)" : "var(--brand)" }} />
          </div>
          <div className="cn-kpi-foot">
            {quota > 0 ? `额度 ${fmt(quota)} · 已用 ${rate.toFixed(1)}%` : "还没有分配给你的 Key"}
          </div>
        </div>

        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="ok">
              <Icon name="activity" size={13} />
            </span>
            调用次数
          </div>
          <div className="cn-kpi-val">
            {fmtNum(monthCalls.length)} <small>次</small>
          </div>
          <div className="cn-kpi-foot">
            {monthCalls.length ? `平均 ${fmt(Math.round(monthSpend / monthCalls.length))} / 次` : "本月还没有调用"}
          </div>
          <Spark values={trend.map((d) => d.total)} color="#12805c" />
        </div>

        <div className="cn-kpi">
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico">
              <Icon name="clock" size={13} />
            </span>
            平均延迟
          </div>
          <div className="cn-kpi-val">
            {avgLatency} <small>ms</small>
          </div>
          <div className="cn-kpi-foot">P95 {p95}ms</div>
        </div>

        <div className="cn-kpi" data-sev={pending.length > 0 ? "warn" : undefined}>
          <div className="cn-kpi-label">
            <span className="cn-kpi-ico" data-t="warn">
              <Icon name="inbox" size={13} />
            </span>
            配额申请
          </div>
          <div className="cn-kpi-val">
            {pending.length} <small>件待审批</small>
          </div>
          <div className="cn-kpi-foot">
            {pending[0]
              ? `${formatDate(pending[0].CreatedAt)} 提交 ${fmt(pending[0].AmountCents)}`
              : "没有待处理的申请"}
          </div>
          <button className="cn-kpi-action" onClick={() => navigate("/app/quota-requests")}>
            {pending.length > 0 ? "查看申请" : "发起申请"} <Icon name="arrow-right" size={12} />
          </button>
        </div>
      </div>

      <div className="cn-grid">
        <TrendCard days={days} trend={trend} />

        <Card title="按模型" note="本月" flush={false}>
          {byModel.length === 0 ? (
            <Empty icon="layers" title="本月还没有调用" desc="用你的 Key 发起第一次请求后，这里按模型拆开。" />
          ) : (
            <div className="cn-share">
              {byModel.map((m) => (
                <div key={m.id} className="cn-share-row">
                  <span className="cn-share-name">
                    <Brand kind={m.model?.ProviderKind} size={13} />
                    <span>{m.model?.Name ?? "未知模型"}</span>
                  </span>
                  <span className="cn-share-pct">{fmtNum(m.calls)}</span>
                  <span className="cn-share-val">{money(m.spend)}</span>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card title="我的 Key" note={`${myKeys.length} 个`}>
          {myKeys.length === 0 ? (
            <Empty icon="key" title="还没有 Key" desc="向管理员申请一把 Key 才能通过网关发起调用。" />
          ) : (
            myKeys.slice(0, 2).map((k) => (
              <div key={k.ID}>
                <div className="cn-kv">
                  <span className="cn-kv-k">名称</span>
                  <span className="cn-kv-v" style={{ fontFamily: "inherit" }}>
                    {k.Name}
                  </span>
                </div>
                <div className="cn-kv">
                  <span className="cn-kv-k">前缀</span>
                  <span className="cn-kv-v">{k.SecretPrefix}••••••••</span>
                  <Button tone="icon"
                    title="复制前缀"
                    onClick={() =>
                      void navigator.clipboard.writeText(k.SecretPrefix).then(() => toast.success("已复制前缀"))
                    }
                  >
                    <Icon name="copy" size={13} />
                  </Button>
                </div>
                <div className="cn-kv">
                  <span className="cn-kv-k">模型范围</span>
                  <span className="cn-kv-v" style={{ fontFamily: "inherit", fontSize: 12 }}>
                    {k.ModelScope && k.ModelScope.length > 0
                      ? k.ModelScope.map((id) => modelById.get(id)?.Name ?? id.slice(0, 8)).join(" · ")
                      : "不限"}
                  </span>
                </div>
              </div>
            ))
          )}
        </Card>
      </div>
    </div>
  )
}

function TrendCard({ days, trend }: { days: number; trend: { day: Date; total: number }[] }) {
  const id = useId()
  const W = 620
  const H = 150
  const values = trend.map((d) => d.total)
  const max = Math.max(1, ...values)
  const total = values.reduce((s, v) => s + v, 0)

  return (
    <Card className="cn-span-2" title="用量趋势" note={`近 ${days} 天 · 按日`} flush={false}>
      <div className="cn-chart-top">
        <span className="cn-chart-big">{fmt(total)}</span>
      </div>
      <div className="cn-plot">
        <div className="cn-yaxis" style={{ height: H }}>
          {[max, max * 0.5, 0].map((t, i) => (
            <span key={i}>{fmtShort(t)}</span>
          ))}
        </div>
        <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} preserveAspectRatio="none">
          <defs>
            <linearGradient id={`my${id}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#2f5fea" stopOpacity=".22" />
              <stop offset="100%" stopColor="#2f5fea" stopOpacity=".01" />
            </linearGradient>
          </defs>
          {[0, 0.5, 1].map((f) => (
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
          <path d={`${smoothPathScaled(values, max, W, H, 8)} L${W} ${H} L0 ${H} Z`} fill={`url(#my${id})`} />
          <path d={smoothPathScaled(values, max, W, H, 8)} fill="none" stroke="#2f5fea" strokeWidth="2" />
        </svg>
        <div className="cn-xaxis">
          {[0, Math.floor(days / 3), Math.floor((days * 2) / 3), days - 1].map((i, n, arr) => (
            <span key={i}>{n === arr.length - 1 ? "今天" : formatDate(trend[i]?.day.toISOString() ?? "")}</span>
          ))}
        </div>
      </div>
    </Card>
  )
}
