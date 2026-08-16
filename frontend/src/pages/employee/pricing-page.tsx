import { useState } from "react"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Filters, PageHead, Select, TableState } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { fmt } from "@/lib/format"
import type { Model } from "@/lib/types"

// 资费一览 -- read-only list page. Only published models appear here,
// which is the same set a Key is allowed to scope to.
export function PricingPage() {
  const models = useApiQuery<Model[]>("/api/models/published")
  const [q, setQ] = useState("")
  const [kind, setKind] = useState("")

  const kinds = [...new Set((models.data ?? []).map((m) => m.ProviderKind).filter(Boolean))] as string[]

  const rows = (models.data ?? []).filter((m) => {
    if (q && !`${m.Name}${m.ModelIdentifier}`.toLowerCase().includes(q.toLowerCase())) return false
    if (kind && m.ProviderKind !== kind) return false
    return true
  })

  return (
    <div className="cn-page">
      <PageHead title="资费一览" sub="按 100 万 token 计价。实际扣费按你这次请求的真实 token 数折算" />

      <Filters
        placeholder="搜索模型…"
        value={q}
        onValue={setQ}
        right={<span className="cn-count">{rows.length} 个可用模型</span>}
      >
        <Select
          label="供应商"
          value={kind}
          onValue={setKind}
          options={[{ value: "", label: "全部供应商" }, ...kinds.map((k) => ({ value: k, label: k }))]}
        />
      </Filters>

      <Card>
        <table className="cn-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>模型标识</th>
              <th className="cn-r">输入 / 1M token</th>
              <th className="cn-r">输出 / 1M token</th>
              <th className="cn-r">上下文</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((m) => (
              <tr key={m.ID}>
                <td>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontWeight: 570 }}>
                    <Brand kind={m.ProviderKind} size={14} />
                    {m.Name}
                  </span>
                </td>
                <td className="cn-mono-cell">{m.ModelIdentifier}</td>
                <td className="cn-r cn-mono" style={{ fontWeight: 560 }}>
                  {fmt(m.InputPriceCentsPer1M)}
                </td>
                <td className="cn-r cn-mono" style={{ fontWeight: 560 }}>
                  {fmt(m.OutputPriceCentsPer1M)}
                </td>
                <td className="cn-r cn-mono" style={{ color: "var(--ink-2)" }}>
                  {(m.ContextWindow / 1000).toFixed(0)}K
                </td>
              </tr>
            ))}
            <TableState
              colSpan={5}
              loading={models.loading}
              empty={rows.length === 0}
              title="还没有可用模型"
              desc="管理员发布模型后，这里会列出它们的单价。"
            />
          </tbody>
        </table>
      </Card>

      <Card flush={false}>
        <div className="cn-notice">
          <Icon name="wallet" size={14} />
          <span>
            价格由管理员在「模型与路由」里维护，可能随采购成本调整。
            调价只影响调整之后的调用，不会回溯已产生的费用。
          </span>
        </div>
      </Card>
    </div>
  )
}
