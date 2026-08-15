import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { formatCents } from "@/lib/format"
import type { Model } from "@/lib/types"

interface ChatMessage {
  role: "user" | "assistant"
  content: string
}

interface Diagnostics {
  latencyMs: number
  costCents: number
  status: "success" | "failed"
}

export function PlaygroundPage() {
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const [modelId, setModelId] = useState("")
  const [secret, setSecret] = useState("")
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState("")
  const [busy, setBusy] = useState(false)
  const [diag, setDiag] = useState<Diagnostics | null>(null)

  const model = (models ?? []).find((m) => m.ID === modelId)

  const createTestKey = async () => {
    try {
      const res = await api.post<{ key: { ID: string }; secret: string }>("/api/virtual-keys", {
        Name: "Playground 测试 Key",
        OwnerType: "member",
        BudgetCents: 1000,
      })
      setSecret(res.secret)
      toast.success("已创建一个 ¥10 的测试 Key，仅用于本次 Playground 会话")
    } catch {
      toast.error("创建测试 Key 失败，请确认你有 Key 管理权限")
    }
  }

  const send = async () => {
    if (!input.trim() || !model || !secret) return
    const next = [...messages, { role: "user" as const, content: input }]
    setMessages(next)
    setInput("")
    setBusy(true)
    const start = performance.now()
    try {
      const res = await fetch("/v1/chat/completions", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${secret}` },
        body: JSON.stringify({ model: model.ModelIdentifier, messages: next.map((m) => ({ role: m.role, content: m.content })) }),
      })
      const latencyMs = Math.round(performance.now() - start)
      if (!res.ok) {
        setDiag({ latencyMs, costCents: 0, status: "failed" })
        toast.error("调用失败")
        return
      }
      const body = await res.json()
      const reply = body.choices?.[0]?.message?.content ?? "（无返回内容）"
      setMessages((prev) => [...prev, { role: "assistant", content: reply }])
      setDiag({ latencyMs, costCents: 0, status: "success" })
    } catch {
      setDiag({ latencyMs: Math.round(performance.now() - start), costCents: 0, status: "failed" })
      toast.error("请求出错")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <PageHeader title="Playground" />

      <div className="flex flex-wrap items-center gap-2">
        <Select value={modelId} onValueChange={setModelId}>
          <SelectTrigger className="w-[220px]">
            <SelectValue placeholder="选择模型" />
          </SelectTrigger>
          <SelectContent>
            {(models ?? []).map((m) => (
              <SelectItem key={m.ID} value={m.ID}>
                {m.Name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {secret ? (
          <span className="text-[11.5px] text-ok">测试 Key 已就绪</span>
        ) : (
          <>
            <Button variant="outline" onClick={() => void createTestKey()}>
              创建测试 Key
            </Button>
            <Input
              placeholder="或粘贴已有 Key（vk-...）"
              className="w-[220px]"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
            />
          </>
        )}
        <div className="flex-1" />
        <Button variant="outline" onClick={() => setMessages([])}>
          清空对话
        </Button>
      </div>

      <div className="flex flex-1 gap-4">
        <div className="flex flex-1 flex-col">
          <div className="flex flex-1 flex-col gap-3.5 overflow-y-auto pb-1">
            {messages.map((m, i) => (
              <div key={i} className={m.role === "user" ? "flex justify-end" : "flex justify-start"}>
                <div
                  className={
                    m.role === "user"
                      ? "max-w-[78%] rounded-xl rounded-br-[4px] bg-primary px-3.5 py-2.5 text-[12.5px] text-primary-foreground"
                      : "max-w-[78%] rounded-xl rounded-bl-[4px] border border-border bg-card px-3.5 py-2.5 text-[12.5px] text-foreground shadow-[var(--shadow-card)]"
                  }
                >
                  {m.content}
                </div>
              </div>
            ))}
            {messages.length === 0 && <p className="text-xs text-muted-foreground">选择模型、准备好 Key 后，在下方输入消息开始测试</p>}
          </div>
          <div className="mt-3.5 flex gap-2">
            <Input
              placeholder="输入消息，测试这个模型…"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void send()}
            />
            <Button disabled={busy || !input.trim() || !model || !secret} onClick={() => void send()}>
              发送
            </Button>
          </div>
        </div>

        <div className="hidden w-[196px] flex-none flex-col gap-2.5 rounded-lg border border-border bg-card p-3.5 shadow-[var(--shadow-card)] md:flex">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">请求诊断</p>
          <DiagRow k="总耗时" v={diag ? `${diag.latencyMs}ms` : "—"} />
          <DiagRow k="命中模型" v={model?.Name ?? "—"} />
          <DiagRow k="状态" v={diag ? (diag.status === "success" ? "成功" : "失败") : "—"} />
          <DiagRow k="预估费用" v={diag ? formatCents(diag.costCents) : "—"} />
        </div>
      </div>
    </div>
  )
}

function DiagRow({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex items-baseline justify-between gap-2 text-[11.5px]">
      <span className="whitespace-nowrap text-muted-foreground">{k}</span>
      <span className="truncate font-mono tabular-nums text-foreground">{v}</span>
    </div>
  )
}
