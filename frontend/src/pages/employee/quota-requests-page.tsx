import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Empty, Input, PageHead, Tag, Textarea } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { fmt, formatAgo, yuanToMicroCents } from "@/lib/format"
import type { QuotaRequest, VirtualKey } from "@/lib/types"

// 配额申请 -- the form page type. Left: the request. Right: what happened
// to the ones already sent, including the reviewer's note when rejected --
// which is the only place an employee finds out why.

const STATUS = {
  pending: { label: "待审批", tone: "warn" },
  approved: { label: "已通过", tone: "ok" },
  rejected: { label: "已驳回", tone: "bad" },
} as const

export function QuotaRequestsPage() {
  const { member, departmentName } = useAuth()
  const requests = useApiQuery<QuotaRequest[]>("/api/quota-requests/mine")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")

  const [amount, setAmount] = useState("")
  const [reason, setReason] = useState("")
  const [busy, setBusy] = useState(false)

  // Same scoping as 我的用量: an admin's own quota, not the org's.
  const mine = (keys ?? []).filter(
    (k) => k.Status === "active" && k.OwnerType === "member" && k.OwnerMemberID === member?.ID,
  )
  const quota = mine.reduce((s, k) => s + k.BudgetMicroCents, 0)
  const used = mine.reduce((s, k) => s + k.SpentMicroCents, 0)
  const rate = quota > 0 ? (used / quota) * 100 : 0

  const submit = async () => {
    const yuan = Number(amount)
    if (!Number.isFinite(yuan) || yuan <= 0) {
      toast.error("请填写正确的申请金额")
      return
    }
    if (!reason.trim()) {
      toast.error("请写清申请事由")
      return
    }
    setBusy(true)
    try {
      await api.post("/api/quota-requests", {
        ModelID: null,
        AmountMicroCents: yuanToMicroCents(yuan),
        Reason: reason.trim(),
      })
      toast.success("已提交申请，等待审批")
      setAmount("")
      setReason("")
      requests.refetch()
    } catch {
      toast.error("提交失败，请稍后再试")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="配额申请" sub="额度不够时向部门负责人申请追加；通过后从部门额度池划拨" />

      <div className="cn-docs">
        <Card title="发起申请" flush={false}>
          <div className="cn-form">
            <div className="cn-form-row">
              <label className="cn-form-label">当前额度</label>
              <div className="cn-quota-bar">
                <i
                  style={{
                    width: `${Math.min(100, rate)}%`,
                    background: rate > 75 ? "var(--warn)" : "var(--brand)",
                  }}
                />
              </div>
              <div className="cn-input-hint">
                {quota > 0
                  ? `已用 ${fmt(used)} / ${fmt(quota)} · 剩余 ${fmt(Math.max(0, quota - used))}`
                  : "你名下还没有生效中的 Key"}
              </div>
            </div>

            <div className="cn-form-row">
              <label className="cn-form-label" htmlFor="req-amount">
                申请金额 <span>必填</span>
              </label>
              <Input
                id="req-amount"
                inputMode="decimal"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="2000"
              />
              <div className="cn-input-hint">单位为元。超过部门额度池剩余的申请，审批人会看到透支提示。</div>
            </div>

            <div className="cn-form-row">
              <label className="cn-form-label" htmlFor="req-reason">
                申请事由 <span>必填</span>
              </label>
              <Textarea
                id="req-reason"
                rows={4}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="多模态评测集需要额外额度，预计跑 3 轮，每轮约 60 万 token。"
              />
              <div className="cn-input-hint">写清用途和预估量，审批会更快。</div>
            </div>

            <div className="cn-form-row">
              <label className="cn-form-label">审批人</label>
              <div className="cn-static">
                <Icon name="users" size={14} />
                {departmentName ? `${departmentName} 负责人` : "部门负责人"}
                <span style={{ marginLeft: "auto", color: "var(--ink-3)", fontSize: 11.5 }}>由部门归属决定</span>
              </div>
            </div>

            <div className="cn-form-foot">
              <Button tone="primary" disabled={busy} onClick={() => void submit()}>
                <Icon name="send" size={14} />
                {busy ? "提交中…" : "提交申请"}
              </Button>
            </div>
          </div>
        </Card>

        <Card title="我的申请记录" note={`${(requests.data ?? []).length} 条`}>
          {(requests.data ?? []).length === 0 ? (
            <Empty icon="inbox" title="还没有申请记录" desc="额度不够时提一单，通过后立即生效。" />
          ) : (
            <div className="cn-todo">
              {(requests.data ?? []).map((r) => {
                const st = STATUS[r.Status] ?? STATUS.pending
                return (
                  <div key={r.ID} className="cn-todo-item">
                    <div className="cn-todo-top">
                      <span className="cn-todo-title" style={{ fontFamily: "var(--mono)", fontSize: 13.5 }}>
                        {fmt(r.AmountMicroCents)}
                      </span>
                      <Tag tone={st.tone}>{st.label}</Tag>
                      <span className="cn-todo-time">{formatAgo(r.CreatedAt)}</span>
                    </div>
                    <p className="cn-todo-desc">{r.Reason || "未填写事由。"}</p>
                  </div>
                )
              })}
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}
