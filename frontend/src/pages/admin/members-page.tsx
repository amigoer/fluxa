import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Avatar } from "@/components/console/avatar"
import { Icon } from "@/components/console/icon"
import { Card, Field, Filters, Input, Modal, PageHead, Select, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt, formatAgo, yuanToMicroCents } from "@/lib/format"
import type { CallLog, Department, Member, QuotaBalance, Role } from "@/lib/types"

// 成员与部门 -- the master/detail page type: departments on the left,
// that department's members and quota pool on the right.

const STATUS = {
  active: { label: "正常", tone: "ok" },
  pending_review: { label: "待审核", tone: "warn" },
  disabled: { label: "已停用", tone: "bad" },
} as const

export function MembersPage() {
  const departments = useApiQuery<Department[]>("/api/departments")
  const members = useApiQuery<Member[]>("/api/members")
  const { data: roles } = useApiQuery<Role[]>("/api/roles")
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs")
  const canManageDepts = useHasPermission(Permission.OrgManageDepartments)
  const canSeePool = useHasPermission(Permission.OrgApproveDepartmentQuota)

  const [dept, setDept] = useState<string>("all")
  const [q, setQ] = useState("")
  const [roleFilter, setRoleFilter] = useState("")
  const [statusFilter, setStatusFilter] = useState("")
  const [creatingDept, setCreatingDept] = useState(false)
  const [editing, setEditing] = useState<Member | null>(null)
  const [adjusting, setAdjusting] = useState(false)

  const pool = useApiQuery<QuotaBalance>(
    canSeePool && dept !== "all" ? `/api/department-quota-pools/${dept}` : null,
    [dept],
  )

  const roleById = useMemo(() => new Map((roles ?? []).map((r) => [r.ID, r])), [roles])
  const current = (departments.data ?? []).find((d) => d.ID === dept)

  // Per-member month spend and last activity, both rolled up from the
  // call log in one pass.
  const activity = useMemo(() => {
    const monthStart = new Date()
    monthStart.setDate(1)
    monthStart.setHours(0, 0, 0, 0)
    const out = new Map<string, { spend: number; last: string }>()
    for (const c of calls ?? []) {
      const cur = out.get(c.MemberID) ?? { spend: 0, last: "" }
      if (new Date(c.OccurredAt) >= monthStart) cur.spend += c.CostMicroCents
      if (!cur.last || new Date(c.OccurredAt) > new Date(cur.last)) cur.last = c.OccurredAt
      out.set(c.MemberID, cur)
    }
    return out
  }, [calls])

  const counts = useMemo(() => {
    const out = new Map<string, number>()
    for (const m of members.data ?? []) {
      if (m.DepartmentID) out.set(m.DepartmentID, (out.get(m.DepartmentID) ?? 0) + 1)
    }
    return out
  }, [members.data])

  const list = (members.data ?? []).filter((m) => {
    if (dept !== "all" && m.DepartmentID !== dept) return false
    if (q && !`${m.Name}${m.Email ?? ""}${m.Phone ?? ""}`.toLowerCase().includes(q.toLowerCase())) return false
    if (roleFilter && m.RoleID !== roleFilter) return false
    if (statusFilter && m.Status !== statusFilter) return false
    return true
  })

  const approve = async (id: string) => {
    try {
      await api.post(`/api/members/${id}/approve`)
      toast.success("已通过审核")
      members.refetch()
    } catch {
      toast.error("操作失败")
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="成员与部门" sub="部门与成员由身份源同步，本地注册的账号需管理员审核后启用">
        <Button
          onClick={() => {
            members.refetch()
            departments.refetch()
            toast.success("已刷新")
          }}
        >
          <Icon name="refresh" size={14} />
          刷新
        </Button>
        {canManageDepts && (
          <Button tone="primary" onClick={() => setCreatingDept(true)}>
            <Icon name="plus" size={14} />
            新建部门
          </Button>
        )}
      </PageHead>

      <div className="cn-md">
        <Card>
          <div className="cn-md-list">
            <button className="cn-md-item" data-on={dept === "all"} onClick={() => setDept("all")}>
              <div className="cn-md-item-main">
                <div>全部成员</div>
              </div>
              <span className="cn-md-count">{(members.data ?? []).length}</span>
            </button>
            {(departments.data ?? []).map((d) => (
              <button key={d.ID} className="cn-md-item" data-on={d.ID === dept} onClick={() => setDept(d.ID)}>
                <div className="cn-md-item-main">
                  <div>{d.Name}</div>
                  {d.LeadMemberID && (
                    <div className="cn-md-item-sub">
                      负责人 {(members.data ?? []).find((m) => m.ID === d.LeadMemberID)?.Name ?? "—"}
                    </div>
                  )}
                </div>
                <span className="cn-md-count">{counts.get(d.ID) ?? 0}</span>
              </button>
            ))}
            {(departments.data ?? []).length === 0 && (
              <div className="cn-md-item" style={{ color: "var(--ink-3)" }}>
                还没有部门
              </div>
            )}
          </div>
        </Card>

        <div style={{ display: "flex", flexDirection: "column", gap: 14, minWidth: 0 }}>
          {current && canSeePool && (
            <Card flush={false}>
              <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 12.5, color: "var(--ink-3)", marginBottom: 6 }}>{current.Name} 额度池</div>
                  <div className="cn-quota-bar">
                    <i
                      style={{
                        width: `${poolPct(pool.data)}%`,
                        background: poolPct(pool.data) > 90 ? "var(--warn)" : "var(--brand)",
                      }}
                    />
                  </div>
                  <div style={{ marginTop: 7, fontSize: 11.5, color: "var(--ink-3)" }}>
                    {pool.data && pool.data.Total > 0
                      ? `已分配 ${fmt(pool.data.Spoken)} / ${fmt(pool.data.Total)} · 剩余 ${fmt(pool.data.Remaining)}`
                      : "尚未设置额度池"}
                  </div>
                </div>
                {canManageDepts && (
                  <Button onClick={() => setAdjusting(true)}>
                    <Icon name="wallet" size={14} />
                    调整额度池
                  </Button>
                )}
              </div>
            </Card>
          )}

          <Filters
            placeholder="搜索姓名或邮箱…"
            value={q}
            onValue={setQ}
            right={<span className="cn-count">{list.length} 人</span>}
          >
            <Select
              label="角色"
              value={roleFilter}
              onValue={setRoleFilter}
              options={[
                { value: "", label: "全部角色" },
                ...(roles ?? []).map((r) => ({ value: r.ID, label: r.Name })),
              ]}
            />
            <Select
              label="状态"
              value={statusFilter}
              onValue={setStatusFilter}
              options={[
                { value: "", label: "全部状态" },
                { value: "active", label: "正常" },
                { value: "pending_review", label: "待审核" },
                { value: "disabled", label: "已停用" },
              ]}
            />
          </Filters>

          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>成员</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">本月花费</TableHead>
                  <TableHead className="text-right">最近活跃</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((m) => {
                  const a = activity.get(m.ID)
                  const st = STATUS[m.Status] ?? STATUS.active
                  return (
                    <TableRow key={m.ID}>
                      <TableCell>
                        <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
                          <Avatar name={m.Name} src={m.AvatarURL} size={26} />
                          <div>
                            <div style={{ fontWeight: 570 }}>{m.Name}</div>
                            <div style={{ fontSize: 11, color: "var(--ink-3)", marginTop: 1 }}>
                              {m.Email ?? m.Phone ?? "—"}
                            </div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell style={{ color: "var(--ink-2)" }}>{roleById.get(m.RoleID)?.Name ?? "—"}</TableCell>
                      <TableCell>
                        <Tag tone={st.tone}>{st.label}</Tag>
                      </TableCell>
                      <TableCell className="text-right cn-mono">{a?.spend ? fmt(a.spend) : "—"}</TableCell>
                      <TableCell className="text-right cn-mono" style={{ color: "var(--ink-3)" }}>
                        {a?.last ? formatAgo(a.last) : "—"}
                      </TableCell>
                      <TableCell>
                        <div className="cn-row-acts">
                          {m.Status === "pending_review" && (
                            <Button tone="miniPri" onClick={() => void approve(m.ID)}>
                              通过
                            </Button>
                          )}
                          <Button tone="icon" title="编辑" onClick={() => setEditing(m)}>
                            <Icon name="edit" size={14} />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
                <TableState
                  colSpan={6}
                  loading={members.loading}
                  empty={list.length === 0}
                  title="没有匹配的成员"
                  desc="成员来自身份源同步或本地注册，不在这里手动创建。"
                />
              </TableBody>
            </Table>
          </Card>
        </div>
      </div>

      <NewDepartmentModal
        open={creatingDept}
        onClose={() => setCreatingDept(false)}
        onDone={() => departments.refetch()}
      />
      <EditMemberModal
        member={editing}
        departments={departments.data ?? []}
        roles={roles ?? []}
        onClose={() => setEditing(null)}
        onDone={() => members.refetch()}
      />
      <AdjustPoolModal
        open={adjusting}
        department={current}
        balance={pool.data}
        onClose={() => setAdjusting(false)}
        onDone={() => pool.refetch()}
      />
    </div>
  )
}

function poolPct(b?: QuotaBalance): number {
  if (!b || b.Total <= 0) return 0
  return Math.min(100, (b.Spoken / b.Total) * 100)
}

function NewDepartmentModal({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: () => void }) {
  const [name, setName] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!name.trim()) return
    setBusy(true)
    try {
      await api.post("/api/departments", { name: name.trim() })
      toast.success("已新建部门")
      setName("")
      onDone()
      onClose()
    } catch {
      toast.error("新建失败")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="新建部门"
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button tone="primary" disabled={busy} onClick={() => void submit()}>
            创建
          </Button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="部门名称" optional="必填">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="研发中心" />
        </Field>
      </div>
    </Modal>
  )
}

function EditMemberModal({
  member,
  departments,
  roles,
  onClose,
  onDone,
}: {
  member: Member | null
  departments: Department[]
  roles: Role[]
  onClose: () => void
  onDone: () => void
}) {
  const [dept, setDept] = useState("")
  const [role, setRole] = useState("")
  const [email, setEmail] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Seed from the member being opened rather than from state, so
  // reopening the dialog on someone else never shows the last person's
  // values.
  const deptValue = dept || member?.DepartmentID || ""
  const roleValue = role || member?.RoleID || ""
  const emailValue = email ?? member?.Email ?? ""

  const submit = async () => {
    if (!member) return
    setBusy(true)
    try {
      if (deptValue !== (member.DepartmentID ?? "")) {
        await api.patch(`/api/members/${member.ID}/department`, { departmentId: deptValue || null })
      }
      if (roleValue !== member.RoleID) {
        await api.patch(`/api/members/${member.ID}/role`, { roleId: roleValue })
      }
      if (emailValue !== (member.Email ?? "")) {
        await api.patch(`/api/members/${member.ID}/contact`, { email: emailValue })
      }
      toast.success("已保存")
      setDept("")
      setRole("")
      setEmail(null)
      onDone()
      onClose()
    } catch {
      toast.error("保存失败")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={!!member}
      title={`编辑 ${member?.Name ?? ""}`}
      sub="部门决定额度池归属，角色决定权限，邮箱决定这个人还能不能登录"
      onClose={() => {
        setDept("")
        setRole("")
        setEmail(null)
        onClose()
      }}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button tone="primary" disabled={busy} onClick={() => void submit()}>
            保存
          </Button>
        </>
      }
    >
      <div className="cn-form">
        <Field
          label="邮箱"
          hint={
            member && !member.Email
              ? "这个成员没有邮箱：身份源没返回过。一旦该身份源被停用，他将无法用任何方式登录。"
              : "同时是登录标识：换成新地址后，验证码会发往新地址。"
          }
        >
          <Input
            value={emailValue}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="未填写"
          />
        </Field>
        <Field label="部门">
          <Select
            variant="input"
            label="部门"
            value={deptValue}
            onValue={setDept}
            options={[
              { value: "", label: "未分配" },
              ...departments.map((d) => ({ value: d.ID, label: d.Name })),
            ]}
          />
        </Field>
        <Field label="角色">
          <Select
            variant="input"
            label="角色"
            value={roleValue}
            onValue={setRole}
            options={roles.map((r) => ({ value: r.ID, label: r.Name }))}
          />
        </Field>
      </div>
    </Modal>
  )
}

function AdjustPoolModal({
  open,
  department,
  balance,
  onClose,
  onDone,
}: {
  open: boolean
  department?: Department
  balance?: QuotaBalance
  onClose: () => void
  onDone: () => void
}) {
  const [total, setTotal] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!department) return
    const yuan = Number(total)
    if (!Number.isFinite(yuan) || yuan < 0) {
      toast.error("请填写正确的金额")
      return
    }
    setBusy(true)
    try {
      await api.put(`/api/department-quota-pools/${department.ID}`, { totalMicroCents: yuanToMicroCents(yuan) })
      toast.success("已更新额度池")
      setTotal("")
      onDone()
      onClose()
    } catch {
      toast.error("更新失败")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title={`调整 ${department?.Name ?? ""} 额度池`}
      sub="部门额度池是审批配额申请时划拨的来源"
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button tone="primary" disabled={busy} onClick={() => void submit()}>
            保存
          </Button>
        </>
      }
    >
      <div className="cn-form">
        <Field
          label="额度池总额"
          optional="必填"
          hint={
            balance
              ? `当前已被 Key 占用 ${fmt(balance.Spoken)}，总额低于这个数会让剩余为负。`
              : "单位为元。"
          }
        >
          <Input
            inputMode="decimal"
            value={total}
            onChange={(e) => setTotal(e.target.value)}
            placeholder={balance ? String(balance.Total / 100) : "0"}
          />
        </Field>
      </div>
    </Modal>
  )
}
