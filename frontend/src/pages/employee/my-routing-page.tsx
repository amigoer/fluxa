import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Empty, Field, Input, Modal, PageHead, Select } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt } from "@/lib/format"
import type { Model, RoutingRule } from "@/lib/types"

// 我的路由配置 -- the chain page type, employee side. Personal rules win
// over the global chain; anything that misses every personal rule falls
// back to whatever the admin configured.
export function MyRoutingPage() {
  const rules = useApiQuery<RoutingRule[]>("/api/routing/mine")
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const canSeeGlobal = useHasPermission(Permission.ProviderManageRouting)
  const global = useApiQuery<RoutingRule[]>(canSeeGlobal ? "/api/routing/global" : null)
  const [adding, setAdding] = useState(false)

  const modelById = useMemo(() => new Map((models ?? []).map((m) => [m.ID, m])), [models])
  const nameOf = (id?: string | null) => (id ? (modelById.get(id)?.Name ?? "—") : null)

  return (
    <div className="cn-page">
      <PageHead title="我的路由配置" sub="个人规则优先于全局规则；没有命中的请求会回落到管理员配置的全局链路">
        <Button tone="primary" onClick={() => setAdding(true)}>
          <Icon name="plus" size={14} />
          新增规则
        </Button>
      </PageHead>

      <Card title="个人规则" note="自上而下匹配，命中即停">
        {(rules.data ?? []).length === 0 ? (
          <Empty
            icon="waypoints"
            title="还没有个人规则"
            desc="没有个人规则时，你的请求直接走管理员配置的全局链路。"
          />
        ) : (
          <div className="cn-chain">
            {(rules.data ?? []).map((r) => (
              <div key={r.ID} className="cn-chain-row">
                <div className="cn-chain-node" data-t="cond">
                  <div className="cn-chain-label">当我</div>
                  <div className="cn-chain-value">{r.ConditionLabel}</div>
                </div>
                <span className="cn-chain-arrow">
                  <Icon name="arrow-right" size={16} />
                </span>
                <div className="cn-chain-node" data-t="target" style={{ width: 214 }}>
                  <div className="cn-chain-label">就用</div>
                  <div className="cn-chain-value">
                    {nameOf(r.TargetModelID)}
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
                  <div className="cn-chain-label">它不可用时</div>
                  <div className="cn-chain-value">{nameOf(r.FallbackModelID) ?? <em>回落全局规则</em>}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card title="全局兜底" note="管理员配置 · 只读">
        {canSeeGlobal && (global.data ?? []).length > 0 ? (
          <div className="cn-chain">
            {(global.data ?? []).map((r) => (
              <div key={r.ID} className="cn-chain-row" style={{ opacity: 0.72 }}>
                <div className="cn-chain-node" data-t="cond">
                  <div className="cn-chain-label">条件</div>
                  <div className="cn-chain-value">{r.ConditionLabel}</div>
                </div>
                <span className="cn-chain-arrow">
                  <Icon name="arrow-right" size={16} />
                </span>
                <div className="cn-chain-node" style={{ width: 214 }}>
                  <div className="cn-chain-label">就用</div>
                  <div className="cn-chain-value">{nameOf(r.TargetModelID)}</div>
                </div>
                <span className="cn-chain-arrow">
                  <Icon name="arrow-right" size={16} />
                </span>
                <div className="cn-chain-node" style={{ width: 190 }}>
                  <div className="cn-chain-label">它不可用时</div>
                  <div className="cn-chain-value">{nameOf(r.FallbackModelID) ?? <em>直接返回错误</em>}</div>
                </div>
                <div className="cn-chain-acts">
                  <Icon name="lock" size={14} style={{ color: "var(--ink-3)" }} />
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="cn-card-body">
            <div className="cn-notice">
              <Icon name="lock" size={14} />
              <span>
                全局链路由管理员维护，员工看不到它的细节。以上个人规则都没有命中的请求，会按管理员配置的顺序转发。
              </span>
            </div>
          </div>
        )}
      </Card>

      <AddRuleModal
        open={adding}
        models={models ?? []}
        onClose={() => setAdding(false)}
        onDone={() => rules.refetch()}
      />
    </div>
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
      await api.post("/api/routing/mine", {
        ConditionLabel: label,
        TargetModelID: target,
        FallbackModelID: fallback || null,
        CostCeilingCents: ceiling ? Math.round(Number(ceiling) * 100) : null,
      })
      toast.success("已新增个人规则")
      setLabel("")
      setCeiling("")
      onDone()
      onClose()
    } catch {
      toast.error("新增失败")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="新增个人路由规则"
      sub="个人规则优先于全局规则"
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
        <Field label="当我" optional="必填" hint="写成一句人话，例如「请求 gpt-4 系列」。">
          <Input value={label} onChange={(e) => setLabel(e.target.value)} />
        </Field>
        <Field label="就用" optional="必填">
          <Select
            variant="input"
            label="就用"
            value={target}
            onValue={setTarget}
            placeholder="选择模型"
            options={models.map((m) => ({
              value: m.ID,
              label: m.Name,
              icon: <Brand kind={m.ProviderKind} size={14} />,
            }))}
          />
        </Field>
        <Field label="它不可用时" hint="留空则回落到管理员配置的全局链路。">
          <Select
            variant="input"
            label="它不可用时"
            value={fallback}
            onValue={setFallback}
            options={[
              { value: "", label: "回落全局规则" },
              ...models.map((m) => ({
                value: m.ID,
                label: m.Name,
                icon: <Brand kind={m.ProviderKind} size={14} />,
              })),
            ]}
          />
        </Field>
        <Field label="单次费用上限" hint="单位元，留空表示不限。">
          <Input inputMode="decimal" value={ceiling} onChange={(e) => setCeiling(e.target.value)} />
        </Field>
      </div>
    </Modal>
  )
}
