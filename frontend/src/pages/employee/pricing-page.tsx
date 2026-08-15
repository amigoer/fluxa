import { useState } from "react"
import { PageHeader } from "@/components/shared/page-header"
import { ProviderAvatar } from "@/components/shared/provider-avatar"
import { Input } from "@/components/ui/input"
import { useApiQuery } from "@/hooks/use-api-query"
import { formatCents } from "@/lib/format"
import type { Model } from "@/lib/types"

export function PricingPage() {
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const [query, setQuery] = useState("")

  const filtered = (models ?? []).filter((m) => m.Name.toLowerCase().includes(query.toLowerCase()))

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="资费一览" />

      <Input placeholder="搜索模型…" value={query} onChange={(e) => setQuery(e.target.value)} className="max-w-[260px]" />

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">模型</th>
              <th className="p-3 font-semibold">输入价格 / 1M</th>
              <th className="p-3 font-semibold">输出价格 / 1M</th>
              <th className="p-3 font-semibold">上下文</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((m) => (
              <tr key={m.ID} className="border-t border-border">
                <td className="p-3">
                  <span className="flex items-center text-foreground">
                    <ProviderAvatar name={m.Name} kind={m.ProviderKind} />
                    {m.Name}
                  </span>
                </td>
                <td className="p-3 font-mono tabular-nums">{formatCents(m.InputPriceCentsPer1M)}</td>
                <td className="p-3 font-mono tabular-nums">{formatCents(m.OutputPriceCentsPer1M)}</td>
                <td className="p-3 text-muted-foreground">{(m.ContextWindow / 1000).toFixed(0)}K</td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无可用模型</p>}
      </div>
    </div>
  )
}
