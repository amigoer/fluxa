import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Empty, Field, Filters, Modal, PageHead, Select, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt } from "@/lib/format"
import type { Model, Provider, RoutingRule } from "@/lib/types"

// 模型与路由 -- list page plus the chain page type. Only published models
// reach an employee's pricing table and a Key's allowed scope, so the two
// live on one screen: the thing being routed and the routing itself.
export function ModelsRoutingPage() {
  const models = useApiQuery<Model[]>("/api/models")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const canManageRouting = useHasPermission(Permission.ProviderManageRouting)
  const canManageModels = useHasPermission(Permission.ProviderManageCredentials)
  const routing = useApiQuery<RoutingRule[]>(canManageRouting ? "/api/routing/global" : null)

  const [q, setQ] = useState("")
  const [providerId, setProviderId] = useState("")
  const [status, setStatus] = useState("")
  const [addingModel, setAddingModel] = useState(false)
  const [addingRule, setAddingRule] = useState(false)

  const providerById = useMemo(() => new Map((providers ?? []).map((p) => [p.ID, p])), [providers])
  const modelById = useMemo(() => new Map((models.data ?? []).map((m) => [m.ID, m])), [models.data])
  const published = (models.data ?? []).filter((m) => m.Status === "published")

  const rows = (models.data ?? []).filter((m) => {
    if (q && !`${m.Name}${m.ModelIdentifier}`.toLowerCase().includes(q.toLowerCase())) return false
    if (providerId && m.ProviderID !== providerId) return false
    if (status && m.Status !== status) return false
    return true
  })

  return (
    <div className="cn-page">
      <PageHead title="模型与路由" sub="已发布的模型才会出现在员工的资费表和 Key 的可选范围里">
        {canManageRouting && (
          <button className="cn-btn" onClick={() => setAddingRule(true)}>
            <Icon name="waypoints" size={14} />
            新增路由规则
          </button>
        )}
        {canManageModels && (
          <button className="cn-btn cn-btn-pri" onClick={() => setAddingModel(true)}>
            <Icon name="plus" size={14} />
            添加模型
          </button>
        )}
      </PageHead>

      <Filters
        placeholder="搜索模型名或标识…"
        value={q}
        onValue={setQ}
        right={
          <span className="cn-count">
            {(models.data ?? []).length} 个模型 · {published.length} 个已发布
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
          label="状态"
          value={status}
          onValue={setStatus}
          options={[
            { value: "", label: "全部状态" },
            { value: "published", label: "已发布" },
            { value: "draft", label: "草稿" },
          ]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>模型标识</th>
              <th>供应商</th>
              <th>状态</th>
              <th className="cn-r">输入 / 1M</th>
              <th className="cn-r">输出 / 1M</th>
              <th className="cn-r">上下文</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((m) => {
              const p = providerById.get(m.ProviderID)
              return (
                <tr key={m.ID}>
                  <td style={{ fontWeight: 570 }}>{m.Name}</td>
                  <td className="cn-mono-cell">{m.ModelIdentifier}</td>
                  <td>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 7, color: "var(--ink-2)" }}>
                      <Brand kind={p?.Kind} size={13} />
                      {p?.Name ?? "—"}
                    </span>
                  </td>
                  <td>
                    <Tag tone={m.Status === "published" ? "ok" : "warn"}>
                      {m.Status === "published" ? "已发布" : "草稿"}
                    </Tag>
                  </td>
                  <td className="cn-r cn-mono">{fmt(m.InputPriceCentsPer1M)}</td>
                  <td className="cn-r cn-mono">{fmt(m.OutputPriceCentsPer1M)}</td>
                  <td className="cn-r cn-mono" style={{ color: "var(--ink-2)" }}>
                    {(m.ContextWindow / 1000).toFixed(0)}K
                  </td>
                </tr>
              )
            })}
            <TableState
              colSpan={7}
              loading={models.loading}
              empty={rows.length === 0}
              title={q || providerId || status ? "没有匹配的模型" : "还没有添加模型"}
              desc="模型定价按 100 万 token 计，是员工资费表的唯一来源。"
            />
          </tbody>
        </table>
      </Card>

      {canManageRouting && (
        <Card title="全局路由规则" note="自上而下匹配，命中即停">
          {(routing.data ?? []).length === 0 ? (
            <Empty
              icon="waypoints"
              title="还没有全局路由规则"
              desc="没有规则时，请求按 Key 指定的模型直接转发。"
            />
          ) : (
            <div className="cn-chain">
              {(routing.data ?? []).map((r) => (
                <div key={r.ID} className="cn-chain-row">
                  <div className="cn-chain-node" data-t="cond">
                    <div className="cn-chain-label">条件</div>
                    <div className="cn-chain-value">{r.ConditionLabel}</div>
                  </div>
                  <span className="cn-chain-arrow">
                    <Icon name="arrow-right" size={16} />
                  </span>
                  <div className="cn-chain-node" data-t="target" style={{ width: 208 }}>
                    <div className="cn-chain-label">目标模型</div>
                    <div className="cn-chain-value">
                      {modelById.get(r.TargetModelID)?.Name ?? "—"}
                      {r.CostCeilingCents && (
                        <span className="cn-chain-ceiling">
                          <Icon name="wallet" size={10} />≤ {fmt(r.CostCeilingCents)}/次
                        </span>
                      )}
                    </div>
                  </div>
                  <span className="cn-chain-arrow">
                    <Icon name="arrow-right" size={16} />
                  </span>
                  <div className="cn-chain-node" style={{ width: 190 }}>
                    <div className="cn-chain-label">备用模型</div>
                    <div className="cn-chain-value">
                      {r.FallbackModelID ? (
                        (modelById.get(r.FallbackModelID)?.Name ?? "—")
                      ) : (
                        <em>无 · 直接返回错误</em>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>
      )}

      <AddModelModal
        open={addingModel}
        providers={providers ?? []}
        onClose={() => setAddingModel(false)}
        onDone={() => models.refetch()}
      />
      <AddRuleModal
        open={addingRule}
        models={published}
        onClose={() => setAddingRule(false)}
        onDone={() => routing.refetch()}
      />
    </div>
  )
}

function AddModelModal({
  open,
  providers,
  onClose,
  onDone,
}: {
  open: boolean
  providers: Provider[]
  onClose: () => void
  onDone: () => void
}) {
  const [providerId, setProviderId] = useState("")
  const [name, setName] = useState("")
  const [identifier, setIdentifier] = useState("")
  const [input, setInput] = useState("")
  const [output, setOutput] = useState("")
  const [ctx, setCtx] = useState("128000")
  const [publish, setPublish] = useState(true)
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!providerId || !name || !identifier) {
      toast.error("请填写供应商、名称和模型标识")
      return
    }
    setBusy(true)
    try {
      await api.post("/api/models", {
        ProviderID: providerId,
        Name: name,
        ModelIdentifier: identifier,
        Status: publish ? "published" : "draft",
        InputPriceCentsPer1M: Math.round(Number(input || 0) * 100),
        OutputPriceCentsPer1M: Math.round(Number(output || 0) * 100),
        ContextWindow: Number(ctx || 0),
      })
      toast.success("已添加模型")
      setName("")
      setIdentifier("")
      setInput("")
      setOutput("")
      onDone()
      onClose()
    } catch {
      toast.error("添加失败，请检查填写内容")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="添加模型"
      sub="定价按 100 万 token 计，单位为元"
      onClose={onClose}
      footer={
        <>
          <button className="cn-btn" onClick={onClose}>
            取消
          </button>
          <button className="cn-btn cn-btn-pri" disabled={busy} onClick={() => void submit()}>
            {busy ? "添加中…" : "添加"}
          </button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="供应商" optional="必填">
          <select className="cn-input" value={providerId} onChange={(e) => setProviderId(e.target.value)}>
            <option value="">选择供应商</option>
            {providers.map((p) => (
              <option key={p.ID} value={p.ID}>
                {p.Name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="展示名称" optional="必填" hint="员工在资费表里看到的名字。">
          <input className="cn-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="GPT-4o" />
        </Field>
        <Field label="模型标识" optional="必填" hint="转发给上游时使用的 model 值。">
          <input
            className="cn-input"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            placeholder="gpt-4o"
          />
        </Field>
        <Field label="输入价格 / 1M token">
          <input className="cn-input" inputMode="decimal" value={input} onChange={(e) => setInput(e.target.value)} placeholder="18.00" />
        </Field>
        <Field label="输出价格 / 1M token">
          <input className="cn-input" inputMode="decimal" value={output} onChange={(e) => setOutput(e.target.value)} placeholder="72.00" />
        </Field>
        <Field label="上下文窗口" hint="单位 token。">
          <input className="cn-input" inputMode="numeric" value={ctx} onChange={(e) => setCtx(e.target.value)} />
        </Field>
        <Field label="状态" hint="草稿不会出现在员工资费表和 Key 的可选范围里。">
          <select className="cn-input" value={publish ? "published" : "draft"} onChange={(e) => setPublish(e.target.value === "published")}>
            <option value="published">已发布</option>
            <option value="draft">草稿</option>
          </select>
        </Field>
      </div>
    </Modal>
  )
}

function AddRuleModal({
  open,
  models,
  onClose,
  onDone,
}: {
  open: boolean
  models: Model[]
  onClose: () => void
  onDone: () => void
}) {
  const [label, setLabel] = useState("")
  const [target, setTarget] = useState("")
  const [fallback, setFallback] = useState("")
  const [ceiling, setCeiling] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!label || !target) {
      toast.error("请填写条件并选择目标模型")
      return
    }
    setBusy(true)
    try {
      await api.post("/api/routing/global", {
        ConditionLabel: label,
        TargetModelID: target,
        FallbackModelID: fallback || null,
        CostCeilingCents: ceiling ? Math.round(Number(ceiling) * 100) : null,
      })
      toast.success("已新增路由规则")
      setLabel("")
      setCeiling("")
      onDone()
      onClose()
    } catch {
      toast.error("新增失败，请检查填写内容")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="新增全局路由规则"
      sub="规则自上而下匹配，命中即停"
      onClose={onClose}
      footer={
        <>
          <button className="cn-btn" onClick={onClose}>
            取消
          </button>
          <button className="cn-btn cn-btn-pri" disabled={busy} onClick={() => void submit()}>
            {busy ? "保存中…" : "保存"}
          </button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="条件" optional="必填" hint="写成一句人话，例如「请求 gpt-4 系列」。">
          <input className="cn-input" value={label} onChange={(e) => setLabel(e.target.value)} />
        </Field>
        <Field label="目标模型" optional="必填">
          <select className="cn-input" value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="">选择模型</option>
            {models.map((m) => (
              <option key={m.ID} value={m.ID}>
                {m.Name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="备用模型" hint="留空则目标不可用时直接返回错误。">
          <select className="cn-input" value={fallback} onChange={(e) => setFallback(e.target.value)}>
            <option value="">无</option>
            {models.map((m) => (
              <option key={m.ID} value={m.ID}>
                {m.Name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="单次费用上限" hint="单位元，留空表示不限。超过上限的请求会走备用模型。">
          <input className="cn-input" inputMode="decimal" value={ceiling} onChange={(e) => setCeiling(e.target.value)} />
        </Field>
      </div>
    </Modal>
  )
}
