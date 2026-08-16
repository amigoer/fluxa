import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Icon } from "@/components/console/icon"
import { Card, Field, Filters, Modal, PageHead, Select, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt, formatDay } from "@/lib/format"
import type { Department, Member, Model, VirtualKey } from "@/lib/types"

// Key 管理 -- list page. A virtual key is the only credential an employee
// ever holds: it carries a model scope and a budget, and the upstream
// provider key never leaves the gateway.
export function KeysPage() {
  const keys = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const { data: members } = useApiQuery<Member[]>("/api/members")
  const { data: departments } = useApiQuery<Department[]>("/api/departments")
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const canManage = useHasPermission(Permission.OrgManageKeys)

  const [q, setQ] = useState("")
  const [owner, setOwner] = useState("")
  const [status, setStatus] = useState("")
  const [creating, setCreating] = useState(false)
  const [issued, setIssued] = useState<{ name: string; secret: string } | null>(null)

  const memberById = useMemo(() => new Map((members ?? []).map((m) => [m.ID, m])), [members])
  const deptById = useMemo(() => new Map((departments ?? []).map((d) => [d.ID, d])), [departments])
  const modelById = useMemo(() => new Map((models ?? []).map((m) => [m.ID, m])), [models])

  const rows = (keys.data ?? []).filter((k) => {
    if (q && !`${k.Name}${k.SecretPrefix}`.toLowerCase().includes(q.toLowerCase())) return false
    if (owner && k.OwnerType !== owner) return false
    if (status && k.Status !== status) return false
    return true
  })
  const activeCount = (keys.data ?? []).filter((k) => k.Status === "active").length

  const ownerName = (k: VirtualKey) =>
    k.OwnerType === "department"
      ? (deptById.get(k.OwnerDepartmentID ?? "")?.Name ?? "未知部门")
      : (memberById.get(k.OwnerMemberID ?? "")?.Name ?? "未知成员")

  const revoke = async (k: VirtualKey) => {
    if (!confirm(`吊销「${k.Name}」？使用这把 Key 的调用会立即开始失败。`)) return
    try {
      await api.post(`/api/virtual-keys/${k.ID}/revoke`)
      toast.success("已吊销")
      keys.refetch()
    } catch {
      toast.error("吊销失败")
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="Key 管理" sub="虚拟 Key 决定可用模型范围和预算上限，不暴露供应商的真实凭证">
        {canManage && (
          <button className="cn-btn cn-btn-pri" onClick={() => setCreating(true)}>
            <Icon name="plus" size={14} />
            签发 Key
          </button>
        )}
      </PageHead>

      <Filters
        placeholder="搜索 Key 名称或前缀…"
        value={q}
        onValue={setQ}
        right={
          <span className="cn-count">
            {activeCount} 个生效 · {(keys.data ?? []).length} 个总计
          </span>
        }
      >
        <Select
          label="归属"
          value={owner}
          onValue={setOwner}
          options={[
            { value: "", label: "全部归属" },
            { value: "member", label: "成员" },
            { value: "department", label: "部门" },
          ]}
        />
        <Select
          label="状态"
          value={status}
          onValue={setStatus}
          options={[
            { value: "", label: "全部状态" },
            { value: "active", label: "生效中" },
            { value: "revoked", label: "已吊销" },
          ]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>Key</th>
              <th>归属</th>
              <th>模型范围</th>
              <th>预算使用</th>
              <th>状态</th>
              <th className="cn-r">签发日期</th>
              <th className="cn-r">操作</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((k) => {
              const rate = k.BudgetCents > 0 ? (k.SpentCents / k.BudgetCents) * 100 : 0
              return (
                <tr key={k.ID} style={k.Status === "revoked" ? { opacity: 0.6 } : undefined}>
                  <td>
                    <div style={{ fontWeight: 570 }}>{k.Name}</div>
                    <div className="cn-mono-cell" style={{ color: "var(--ink-3)", marginTop: 2 }}>
                      {k.SecretPrefix}••••••••
                    </div>
                  </td>
                  <td>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-2)" }}>
                      <Icon name={k.OwnerType === "department" ? "users" : "key"} size={13} />
                      {ownerName(k)}
                    </span>
                  </td>
                  <td style={{ color: "var(--ink-2)" }}>
                    {k.ModelScope && k.ModelScope.length > 0 ? (
                      <span className="cn-trunc" style={{ maxWidth: 190 }}>
                        {k.ModelScope.map((id) => modelById.get(id)?.Name ?? id.slice(0, 8)).join(" · ")}
                      </span>
                    ) : (
                      <span style={{ color: "var(--ink-3)" }}>不限</span>
                    )}
                  </td>
                  <td>
                    <div className="cn-usage">
                      <span className="cn-usage-track">
                        <i data-over={rate > 90} style={{ width: `${Math.min(100, rate)}%` }} />
                      </span>
                      <span className="cn-mono" style={{ color: rate > 90 ? "var(--warn)" : "var(--ink-3)" }}>
                        {rate.toFixed(0)}%
                      </span>
                    </div>
                    <div className="cn-mono-cell" style={{ color: "var(--ink-3)", marginTop: 3 }}>
                      {fmt(k.SpentCents)} / {fmt(k.BudgetCents)}
                    </div>
                  </td>
                  <td>
                    <Tag tone={k.Status === "active" ? "ok" : "bad"}>
                      {k.Status === "active" ? "生效中" : "已吊销"}
                    </Tag>
                  </td>
                  <td className="cn-r cn-mono" style={{ color: "var(--ink-3)" }}>
                    {formatDay(k.CreatedAt)}
                  </td>
                  <td>
                    <div className="cn-row-acts">
                      <button
                        className="cn-icon-act"
                        title="复制前缀"
                        onClick={() =>
                          void navigator.clipboard.writeText(k.SecretPrefix).then(() => toast.success("已复制前缀"))
                        }
                      >
                        <Icon name="copy" size={14} />
                      </button>
                      {k.Status === "active" && (
                        <button
                          className="cn-icon-act"
                          data-danger="true"
                          title="吊销"
                          onClick={() => void revoke(k)}
                        >
                          <Icon name="trash" size={14} />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              )
            })}
            <TableState
              colSpan={7}
              loading={keys.loading}
              empty={rows.length === 0}
              title="还没有签发 Key"
              desc="没有 Key，员工就无法通过网关发起任何调用。"
            />
          </tbody>
        </table>
      </Card>

      <IssueKeyModal
        open={creating}
        members={members ?? []}
        departments={departments ?? []}
        models={models ?? []}
        onClose={() => setCreating(false)}
        onIssued={(name, secret) => {
          setIssued({ name, secret })
          keys.refetch()
        }}
      />

      <Modal
        open={!!issued}
        title="Key 已签发"
        sub="明文只显示这一次，关掉后无法再取回"
        onClose={() => setIssued(null)}
        footer={
          <button className="cn-btn cn-btn-pri" onClick={() => setIssued(null)}>
            我已保存
          </button>
        }
      >
        <div className="cn-form">
          <Field label={issued?.name ?? "Key"} hint="把它交给使用者，或直接填进他们的 SDK 配置里。">
            <div className="cn-static" style={{ fontFamily: "var(--mono)", wordBreak: "break-all" }}>
              {issued?.secret}
              <button
                className="cn-icon-act"
                style={{ marginLeft: "auto" }}
                title="复制"
                onClick={() =>
                  void navigator.clipboard.writeText(issued?.secret ?? "").then(() => toast.success("已复制"))
                }
              >
                <Icon name="copy" size={14} />
              </button>
            </div>
          </Field>
        </div>
      </Modal>
    </div>
  )
}

function IssueKeyModal({
  open,
  members,
  departments,
  models,
  onClose,
  onIssued,
}: {
  open: boolean
  members: Member[]
  departments: Department[]
  models: Model[]
  onClose: () => void
  onIssued: (name: string, secret: string) => void
}) {
  const [name, setName] = useState("")
  const [ownerType, setOwnerType] = useState<"member" | "department">("member")
  const [ownerId, setOwnerId] = useState("")
  const [budget, setBudget] = useState("")
  const [scope, setScope] = useState<string[]>([])
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    const yuan = Number(budget)
    if (!name || !ownerId || !Number.isFinite(yuan) || yuan <= 0) {
      toast.error("请填写名称、归属和预算")
      return
    }
    setBusy(true)
    try {
      const res = await api.post<{ secret: string }>("/api/virtual-keys", {
        Name: name,
        OwnerType: ownerType,
        OwnerMemberID: ownerType === "member" ? ownerId : null,
        OwnerDepartmentID: ownerType === "department" ? ownerId : null,
        ModelScope: scope.length > 0 ? scope : null,
        BudgetCents: Math.round(yuan * 100),
      })
      onIssued(name, res.secret)
      setName("")
      setBudget("")
      setScope([])
      onClose()
    } catch {
      toast.error("签发失败，请检查填写内容")
    } finally {
      setBusy(false)
    }
  }

  // model_scope is a uuid[] column: it stores model IDs, not the
  // provider-facing identifier string.
  const toggleModel = (id: string) =>
    setScope((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]))

  return (
    <Modal
      open={open}
      title="签发虚拟 Key"
      sub="预算与模型范围都在这把 Key 上，随时可以吊销"
      onClose={onClose}
      footer={
        <>
          <button className="cn-btn" onClick={onClose}>
            取消
          </button>
          <button className="cn-btn cn-btn-pri" disabled={busy} onClick={() => void submit()}>
            {busy ? "签发中…" : "签发"}
          </button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="名称" optional="必填" hint="写清用途和使用者，吊销时才知道动的是哪一把。">
          <input className="cn-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="林见夏 · 评测" />
        </Field>
        <Field label="归属" optional="必填" hint="部门 Key 从部门额度池划扣，成员 Key 只算这个人的账。">
          <select
            className="cn-input"
            value={ownerType}
            onChange={(e) => {
              setOwnerType(e.target.value as "member" | "department")
              setOwnerId("")
            }}
          >
            <option value="member">成员</option>
            <option value="department">部门</option>
          </select>
        </Field>
        <Field label={ownerType === "member" ? "成员" : "部门"} optional="必填">
          <select className="cn-input" value={ownerId} onChange={(e) => setOwnerId(e.target.value)}>
            <option value="">请选择</option>
            {(ownerType === "member" ? members : departments).map((o) => (
              <option key={o.ID} value={o.ID}>
                {o.Name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="预算" optional="必填" hint="单位为元。用满后这把 Key 的调用会被直接拒绝。">
          <input className="cn-input" inputMode="decimal" value={budget} onChange={(e) => setBudget(e.target.value)} placeholder="2000" />
        </Field>
        <Field label="模型范围" hint="不勾选表示不限，可用所有已发布模型。">
          <div className="cn-rows" style={{ gap: 2 }}>
            {models.map((m) => (
              <label
                key={m.ID}
                style={{ display: "flex", alignItems: "center", gap: 8, padding: "5px 0", fontSize: 12.5, cursor: "pointer" }}
              >
                <input
                  type="checkbox"
                  checked={scope.includes(m.ID)}
                  onChange={() => toggleModel(m.ID)}
                />
                {m.Name}
                <span className="cn-mono-cell" style={{ color: "var(--ink-3)", marginLeft: "auto" }}>
                  {m.ModelIdentifier}
                </span>
              </label>
            ))}
            {models.length === 0 && <span className="cn-input-hint">还没有已发布的模型。</span>}
          </div>
        </Field>
      </div>
    </Modal>
  )
}
