import { useEffect, useRef, useState, type ReactNode } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Empty, PageHead, Select, Tag, Textarea } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt, fmtNum } from "@/lib/format"
import type { CallLog, Model, Provider, VirtualKey } from "@/lib/types"

// Playground -- the chat page type. The point of the screen is not the
// conversation, it is the panel on the right: did the request land on the
// model and provider you expected, and what did it cost.
//
// The diagnostics are real, not estimated. The gateway honours an
// inbound X-Request-Id (internal/gateway/pipeline.go), so the page mints
// one, sends it, then finds that exact row in the call log and reads the
// provider, tokens and cost the gateway itself recorded.

interface ChatMessage {
  role: "user" | "assistant"
  content: string
}

interface Diag {
  requestId: string
  latencyMs: number
  status: "success" | "failed"
  log?: CallLog
}

export function PlaygroundPage() {
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const keys = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const canReadLogs = useHasPermission(Permission.AuditViewCallLogs)
  const canMakeKeys = useHasPermission(Permission.OrgManageKeys)

  const [modelId, setModelId] = useState("")
  const [secret, setSecret] = useState("")
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState("")
  const [busy, setBusy] = useState(false)
  const [creatingKey, setCreatingKey] = useState(false)
  const [diag, setDiag] = useState<Diag | null>(null)
  const bodyRef = useRef<HTMLDivElement>(null)

  const model = (models ?? []).find((m) => m.ID === modelId)
  const provider = providers?.find((p) => p.ID === diag?.log?.ProviderID)

  useEffect(() => {
    bodyRef.current?.scrollTo({ top: bodyRef.current.scrollHeight, behavior: "smooth" })
  }, [messages])

  const createTestKey = async () => {
    if (creatingKey) return
    setCreatingKey(true)
    try {
      const res = await api.post<{ key: { ID: string }; secret: string }>("/api/virtual-keys", {
        Name: "Playground 测试 Key",
        OwnerType: "member",
        BudgetCents: 1000,
      })
      setSecret(res.secret)
      keys.refetch()
      toast.success("已创建一个 ¥10 的测试 Key，仅用于本次会话")
    } catch {
      toast.error("创建失败，请确认你有 Key 管理权限")
    } finally {
      setCreatingKey(false)
    }
  }

  const send = async () => {
    if (!input.trim() || !model || !secret || busy) return
    const next = [...messages, { role: "user" as const, content: input }]
    setMessages(next)
    setInput("")
    setBusy(true)

    const requestId = crypto.randomUUID()
    const started = performance.now()
    try {
      const res = await fetch("/v1/chat/completions", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${secret}`,
          "X-Request-Id": requestId,
        },
        body: JSON.stringify({
          model: model.ModelIdentifier,
          messages: next.map((m) => ({ role: m.role, content: m.content })),
        }),
      })
      const latencyMs = Math.round(performance.now() - started)
      if (!res.ok) {
        setDiag({ requestId, latencyMs, status: "failed" })
        toast.error("调用失败，看右侧诊断和调用日志定位")
        return
      }
      const body = await res.json()
      const reply = body.choices?.[0]?.message?.content ?? "（无返回内容）"
      setMessages((prev) => [...prev, { role: "assistant", content: reply }])
      setDiag({ requestId, latencyMs, status: "success" })

      // The gateway writes the call log after it responds, so give it a
      // beat before looking the row up.
      if (canReadLogs) {
        setTimeout(() => {
          void api
            .get<CallLog[]>("/api/call-logs")
            .then((logs) => {
              const log = logs.find((l) => l.RequestID === requestId)
              if (log) setDiag((d) => (d && d.requestId === requestId ? { ...d, log } : d))
            })
            .catch(() => {})
        }, 600)
      }
    } catch {
      setDiag({ requestId, latencyMs: Math.round(performance.now() - started), status: "failed" })
      toast.error("请求出错")
    } finally {
      setBusy(false)
    }
  }

  const ready = !!model && !!secret

  return (
    <div className="cn-page" style={{ flex: 1, minHeight: 0 }}>
      <PageHead title="Playground" sub="验证路由和 Provider 是否真的按预期生效，右侧给出这次请求的完整诊断">
        <Select
          label="模型"
          value={modelId}
          onValue={setModelId}
          placeholder="选择模型"
          options={(models ?? []).map((m) => ({
            value: m.ID,
            label: m.Name,
            icon: <Brand kind={m.ProviderKind} size={14} />,
          }))}
        />
        <Button onClick={() => { setMessages([]); setDiag(null) }}>
          <Icon name="refresh" size={14} />
          清空对话
        </Button>
      </PageHead>

      {!secret && (
        <Card flush={false}>
          <div className="cn-notice">
            <Icon name="key" size={14} />
            <span>
              Playground 走的是真实网关，需要一个虚拟 Key 才能发请求。
              {canMakeKeys ? "可以直接创建一个 ¥10 的临时 Key，也可以粘贴已有 Key。" : "请向管理员申请一个 Key 并粘贴到这里。"}
            </span>
          </div>
          <div className="cn-filters" style={{ marginTop: 12 }}>
            {canMakeKeys && (
              <Button tone="primary" disabled={creatingKey} onClick={() => void createTestKey()}>
                <Icon name="plus" size={14} />
                {creatingKey ? "创建中…" : "创建测试 Key"}
              </Button>
            )}
            <label className="cn-field" style={{ width: 280 }}>
              <Icon name="key" size={14} />
              <input value={secret} onChange={(e) => setSecret(e.target.value)} placeholder="粘贴已有 Key（sk-flx-…）" />
            </label>
          </div>
        </Card>
      )}

      <div className="cn-play">
        <div className="cn-card cn-chat">
          <div className="cn-card-head">
            <span className="cn-card-title">对话</span>
            <span className="cn-card-note">{model ? `${model.Name} · ${model.ModelIdentifier}` : "未选定模型"}</span>
          </div>
          <div className="cn-chat-body" ref={bodyRef}>
            {messages.length === 0 && (
              <Empty
                icon="terminal"
                title="还没有对话"
                desc={ready ? "在下面输入一条消息，右侧会给出这次请求的完整诊断。" : "先选好模型、准备好 Key，再开始测试。"}
              />
            )}
            {messages.map((m, i) => (
              <div key={i} className="cn-msg" data-role={m.role}>
                <span className="cn-msg-av">
                  {m.role === "user" ? "我" : provider?.Kind ? <Brand kind={provider.Kind} size={13} /> : "AI"}
                </span>
                <div className="cn-msg-body">{m.content}</div>
              </div>
            ))}
            {busy && (
              <div className="cn-msg" data-role="assistant">
                <span className="cn-msg-av">…</span>
                <div className="cn-msg-body" style={{ color: "var(--ink-3)" }}>
                  正在等待上游返回…
                </div>
              </div>
            )}
          </div>
          <div className="cn-chat-input">
            <Textarea
              rows={2}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault()
                  void send()
                }
              }}
              placeholder={ready ? "输入消息，Enter 发送，Shift + Enter 换行" : "先选择模型并准备好 Key"}
            />
            <button className="cn-send" disabled={!ready || busy || !input.trim()} onClick={() => void send()}>
              <Icon name="send" size={16} />
            </button>
          </div>
        </div>

        <Card title="本次请求诊断" note={diag ? diag.requestId.slice(0, 8) : "尚未发起"}>
          {!diag ? (
            <Empty icon="sliders" title="还没有可诊断的请求" desc="发出第一条消息后，这里会显示实际命中的模型和费用。" />
          ) : (
            <div className="cn-diag">
              <DiagRow k="状态">
                <Tag tone={diag.status === "success" ? "ok" : "bad"}>
                  {diag.status === "success" ? "成功" : "失败"}
                </Tag>
              </DiagRow>
              <DiagRow k="请求模型">{model?.ModelIdentifier ?? "—"}</DiagRow>
              <DiagRow k="实际模型">
                <Brand kind={provider?.Kind} size={13} />
                {model?.Name ?? "—"}
              </DiagRow>
              <DiagRow k="供应商">{provider?.Name ?? (diag.log ? "—" : "等待日志…")}</DiagRow>
              <DiagRow k="总耗时" mono>
                {diag.latencyMs}ms
              </DiagRow>
              <DiagRow k="网关耗时" mono>
                {diag.log ? `${diag.log.LatencyMS}ms` : "—"}
              </DiagRow>
              <DiagRow k="Token" mono>
                {diag.log ? `${fmtNum(diag.log.InputTokens)} 入 / ${fmtNum(diag.log.OutputTokens)} 出` : "—"}
              </DiagRow>
              <DiagRow k="本次费用" mono strong>
                {diag.log ? fmt(diag.log.CostCents) : "—"}
              </DiagRow>
              <DiagRow k="Request ID" mono>
                {diag.requestId.slice(0, 18)}…
                <Button tone="icon"
                  title="复制"
                  onClick={() => void navigator.clipboard.writeText(diag.requestId).then(() => toast.success("已复制 Request ID"))}
                >
                  <Icon name="copy" size={13} />
                </Button>
              </DiagRow>
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}

function DiagRow({
  k,
  mono,
  strong,
  children,
}: {
  k: string
  mono?: boolean
  strong?: boolean
  children: ReactNode
}) {
  return (
    <div className="cn-diag-row">
      <span className="cn-diag-k">{k}</span>
      <span
        className={mono ? "cn-diag-v mono" : "cn-diag-v"}
        style={strong ? { color: "var(--ink)", fontWeight: 620 } : undefined}
      >
        {children}
      </span>
    </div>
  )
}
