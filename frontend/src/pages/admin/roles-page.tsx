import { Fragment, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Check, Field, Input, Modal, PageHead } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { permissionCatalog } from "@/lib/permission-catalog"
import type { Member, Role } from "@/lib/types"

// 角色权限 -- master/detail again, with the permission matrix as the
// detail. Showing every role as a column rather than just the selected
// one is the whole point: "who else can do this" is the question an admin
// actually has when they are about to grant something.
export function RolesPage() {
  const roles = useApiQuery<Role[]>("/api/roles")
  const { data: members } = useApiQuery<Member[]>("/api/members")
  const [active, setActive] = useState("")
  const [grants, setGrants] = useState<Record<string, Set<string>>>({})
  const [draft, setDraft] = useState<Set<string> | null>(null)
  const [creating, setCreating] = useState(false)
  const [saving, setSaving] = useState(false)

  const list = roles.data ?? []
  const current = list.find((r) => r.ID === active) ?? list[0]

  useEffect(() => {
    const first = roles.data?.[0]
    if (!active && first) setActive(first.ID)
  }, [active, roles.data])

  // Every role's permission set, one request each. There are a handful of
  // roles, and the matrix is meaningless without all of them.
  useEffect(() => {
    let cancelled = false
    Promise.all(
      (roles.data ?? []).map((r) =>
        api
          .get<string[]>(`/api/roles/${r.ID}/permissions`)
          .then((codes) => [r.ID, new Set(codes ?? [])] as const)
          .catch(() => [r.ID, new Set<string>()] as const),
      ),
    ).then((pairs) => {
      if (cancelled) return
      setGrants(Object.fromEntries(pairs))
    })
    return () => {
      cancelled = true
    }
  }, [roles.data])

  const memberCount = useMemo(() => {
    const out = new Map<string, number>()
    for (const m of members ?? []) out.set(m.RoleID, (out.get(m.RoleID) ?? 0) + 1)
    return out
  }, [members])

  const editable = !!current && !current.IsBuiltin
  const shown = draft ?? (current ? (grants[current.ID] ?? new Set<string>()) : new Set<string>())

  const toggle = (code: string) => {
    if (!editable) return
    const next = new Set(shown)
    if (next.has(code)) next.delete(code)
    else next.add(code)
    setDraft(next)
  }

  const save = async () => {
    if (!current || !draft) return
    setSaving(true)
    try {
      await api.put(`/api/roles/${current.ID}/permissions`, { permissions: [...draft] })
      setGrants((g) => ({ ...g, [current.ID]: new Set(draft) }))
      setDraft(null)
      toast.success("权限已保存")
    } catch {
      toast.error("保存失败")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="角色权限" sub="角色 = 权限点集合。内置角色不可编辑，可按需拆出自定义角色">
        <Button tone="primary" onClick={() => setCreating(true)}>
          <Icon name="plus" size={14} />
          新建角色
        </Button>
      </PageHead>

      <div className="cn-md">
        <Card>
          <div className="cn-md-list">
            {list.map((r) => (
              <button
                key={r.ID}
                className="cn-md-item"
                data-on={r.ID === current?.ID}
                onClick={() => {
                  setActive(r.ID)
                  setDraft(null)
                }}
              >
                <div className="cn-md-item-main">
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    {r.Name}
                    {r.IsBuiltin && <Icon name="lock" size={11} style={{ color: "var(--ink-3)" }} />}
                  </div>
                  <div className="cn-md-item-sub">{r.IsBuiltin ? "内置角色" : "自定义角色"}</div>
                </div>
                <span className="cn-md-count">{memberCount.get(r.ID) ?? 0}</span>
              </button>
            ))}
            {list.length === 0 && (
              <div className="cn-md-item" style={{ color: "var(--ink-3)" }}>
                还没有角色
              </div>
            )}
          </div>
        </Card>

        <Card
          title={current ? `${current.Name} 的权限` : "权限"}
          note={current?.IsBuiltin ? "内置角色 · 只读" : `${memberCount.get(current?.ID ?? "") ?? 0} 名成员`}
          link={editable && draft ? (saving ? "保存中…" : "保存修改") : undefined}
          onLink={() => void save()}
        >
          {current?.IsBuiltin && (
            <div style={{ padding: "12px 16px 0" }}>
              <div className="cn-notice">
                <Icon name="lock" size={14} />
                <span>内置角色的权限固定，不能修改。需要更细的划分时，请新建自定义角色。</span>
              </div>
            </div>
          )}
          <table className="cn-matrix">
            <thead>
              <tr>
                <th>权限点</th>
                {list.map((r) => (
                  <th key={r.ID} style={r.ID === current?.ID ? { color: "var(--brand)" } : undefined}>
                    {r.Name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {permissionCatalog.map((g) => (
                <Fragment key={g.group}>
                  <tr className="cn-matrix-group">
                    <td colSpan={list.length + 1}>{g.group}</td>
                  </tr>
                  {g.items.map((it) => (
                    <tr key={it.code}>
                      <td>
                        {it.label}
                        <span className="cn-matrix-key">{it.code}</span>
                      </td>
                      {list.map((r) => {
                        const isCurrent = r.ID === current?.ID
                        const on = isCurrent ? shown.has(it.code) : (grants[r.ID]?.has(it.code) ?? false)
                        return (
                          <td key={r.ID} style={isCurrent ? { background: "#f7f9ff" } : undefined}>
                            <Check
                              on={on}
                              locked={r.IsBuiltin}
                              label={`${r.Name} · ${it.label}`}
                              onToggle={isCurrent && editable ? () => toggle(it.code) : undefined}
                            />
                          </td>
                        )
                      })}
                    </tr>
                  ))}
                </Fragment>
              ))}
            </tbody>
          </table>
        </Card>
      </div>

      <NewRoleModal open={creating} onClose={() => setCreating(false)} onDone={() => roles.refetch()} />
    </div>
  )
}

function NewRoleModal({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: () => void }) {
  const [name, setName] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!name.trim()) return
    setBusy(true)
    try {
      await api.post("/api/roles", { name: name.trim(), permissions: [] })
      toast.success("已新建角色，接下来勾选权限点")
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
      title="新建角色"
      sub="新角色默认没有任何权限，创建后在矩阵里勾选"
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
        <Field label="角色名称" optional="必填">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="部门负责人" />
        </Field>
      </div>
    </Modal>
  )
}
