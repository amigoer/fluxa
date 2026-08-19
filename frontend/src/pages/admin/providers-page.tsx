import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Field, Filters, Input, Modal, PageHead, Select, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useHasPermission } from "@/lib/auth"
import { fmt, fmtNum } from "@/lib/format"
import { tError } from "@/lib/i18n"
import type { CallLog, Provider, ProviderHealth } from "@/lib/types"

// 供应商 -- the list page type. Circuit state comes from the gateway's own
// health table; latency, failure rate, volume and spend are rolled up from
// the call log, which is the only place those numbers exist.

const HEALTH_LABEL = { normal: "正常", half_open: "半开", circuit_open: "熔断" } as const
const HEALTH_TONE = { normal: "ok", half_open: "warn", circuit_open: "bad" } as const

// The five the gateway can actually forward to. The server refuses
// anything else at /api/providers with a 400, and keeping the two lists
// in step is what stops an admin saving a provider that looks configured
// and healthy on this page and then fails every call at runtime.
//
// 阿里云百炼 and the other domestic vendors are reached through
// openai_compatible: they serve the OpenAI wire format, so they need a
// base URL rather than an adapter of their own.
const KINDS = [
  { value: "openai_compatible", label: "OpenAI 兼容" },
  { value: "anthropic", label: "Anthropic" },
  { value: "azure_openai", label: "Azure OpenAI" },
  { value: "gemini", label: "Google Gemini" },
  { value: "bedrock", label: "AWS Bedrock" },
]

type CredentialField = { key: string; label: string; secret?: boolean; optional?: boolean; hint?: string; placeholder?: string }

// Each vendor authenticates differently, so the form asks for what that
// vendor needs rather than one API-key box that is wrong for two of the
// five. The keys match what the gateway reads out of provider config.
const CREDENTIAL_FIELDS: Record<string, CredentialField[]> = {
  openai_compatible: [{ key: "api_key", label: "API Key", secret: true, placeholder: "sk-…" }],
  anthropic: [{ key: "api_key", label: "API Key", secret: true, placeholder: "sk-ant-…" }],
  azure_openai: [
    { key: "api_key", label: "API Key", secret: true },
    { key: "api_version", label: "API 版本", optional: true, hint: "留空使用默认版本。", placeholder: "2024-10-21" },
  ],
  gemini: [{ key: "api_key", label: "API Key", secret: true, placeholder: "AIza…" }],
  bedrock: [
    { key: "region", label: "区域", placeholder: "us-east-1", hint: "Bedrock 的调用地址按区域区分。" },
    { key: "access_key_id", label: "Access Key ID", secret: true },
    { key: "secret_access_key", label: "Secret Access Key", secret: true },
    { key: "session_token", label: "Session Token", secret: true, optional: true, hint: "仅使用临时凭证时需要。" },
  ],
}

const BASE_URL_PLACEHOLDER: Record<string, string> = {
  openai_compatible: "https://api.openai.com/v1",
  anthropic: "https://api.anthropic.com",
  azure_openai: "https://my-resource.openai.azure.com",
  gemini: "https://generativelanguage.googleapis.com",
  bedrock: "https://bedrock-runtime.us-east-1.amazonaws.com",
}

export function ProvidersPage() {
  const providers = useApiQuery<Provider[]>("/api/providers")
  const health = useApiQuery<ProviderHealth[]>("/api/provider-health")
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs")
  const canCreate = useHasPermission(Permission.ProviderManageCredentials)

  const [q, setQ] = useState("")
  const [kind, setKind] = useState("")
  const [state, setState] = useState("")
  const [creating, setCreating] = useState(false)

  const healthById = useMemo(
    () => new Map((health.data ?? []).map((h) => [h.ProviderID, h])),
    [health.data],
  )

  // One pass over the log per provider: month spend, month calls, p95 and
  // failure rate. Doing it here rather than per-cell keeps the table from
  // re-scanning the same array six times.
  const stats = useMemo(() => {
    const monthStart = new Date()
    monthStart.setDate(1)
    monthStart.setHours(0, 0, 0, 0)
    const acc = new Map<string, { spend: number; calls: number; failed: number; lat: number[] }>()
    for (const c of calls ?? []) {
      if (new Date(c.OccurredAt) < monthStart) continue
      const cur = acc.get(c.ProviderID) ?? { spend: 0, calls: 0, failed: 0, lat: [] }
      cur.spend += c.CostMicroCents
      cur.calls += 1
      if (c.Status === "failed") cur.failed += 1
      cur.lat.push(c.LatencyMS)
      acc.set(c.ProviderID, cur)
    }
    const out = new Map<string, { spend: number; calls: number; errRate: number; p95: number }>()
    for (const [id, v] of acc) {
      const sorted = v.lat.sort((a, b) => a - b)
      out.set(id, {
        spend: v.spend,
        calls: v.calls,
        errRate: v.calls ? (v.failed / v.calls) * 100 : 0,
        p95: sorted.length ? sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))] : 0,
      })
    }
    return out
  }, [calls])

  const rows = (providers.data ?? []).filter((p) => {
    if (q && !p.Name.toLowerCase().includes(q.toLowerCase())) return false
    if (kind && p.Kind !== kind) return false
    if (state && (healthById.get(p.ID)?.State ?? "normal") !== state) return false
    return true
  })

  return (
    <div className="cn-page">
      <PageHead title="供应商" sub="上游账号与凭证，熔断状态实时反映网关探活结果">
        <Button
          onClick={() => {
            providers.refetch()
            health.refetch()
            toast.success("已刷新熔断状态")
          }}
        >
          <Icon name="refresh" size={14} />
          刷新状态
        </Button>
        {canCreate && (
          <Button tone="primary" onClick={() => setCreating(true)}>
            <Icon name="plus" size={14} />
            接入供应商
          </Button>
        )}
      </PageHead>

      <Filters
        placeholder="搜索供应商名称…"
        value={q}
        onValue={setQ}
        right={<span className="cn-count">{rows.length} 个供应商</span>}
      >
        <Select
          label="类型"
          value={kind}
          onValue={setKind}
          options={[{ value: "", label: "全部类型" }, ...KINDS]}
        />
        <Select
          label="熔断状态"
          value={state}
          onValue={setState}
          options={[
            { value: "", label: "全部状态" },
            { value: "normal", label: "正常" },
            { value: "half_open", label: "半开" },
            { value: "circuit_open", label: "熔断" },
          ]}
        />
      </Filters>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>供应商</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>熔断状态</TableHead>
              <TableHead className="text-right">P95 延迟</TableHead>
              <TableHead className="text-right">失败率</TableHead>
              <TableHead className="text-right">本月调用</TableHead>
              <TableHead className="text-right">本月花费</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((p) => {
              const s = stats.get(p.ID)
              const st = healthById.get(p.ID)?.State ?? "normal"
              return (
                <TableRow key={p.ID}>
                  <TableCell>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 9 }}>
                      <span className="cn-cfg-logo" style={{ width: 26, height: 26, borderRadius: 8 }}>
                        <Brand kind={p.Kind} size={14} />
                      </span>
                      <span style={{ fontWeight: 570 }}>{p.Name}</span>
                    </span>
                  </TableCell>
                  <TableCell className="cn-mono-cell" style={{ color: "var(--ink-2)" }}>
                    {p.Kind}
                  </TableCell>
                  <TableCell>
                    <Tag tone={HEALTH_TONE[st]}>{HEALTH_LABEL[st]}</Tag>
                  </TableCell>
                  <TableCell className="text-right cn-mono" style={{ color: (s?.p95 ?? 0) > 2000 ? "var(--bad)" : "var(--ink-2)" }}>
                    {s ? `${s.p95}ms` : "—"}
                  </TableCell>
                  <TableCell
                    className="text-right cn-mono"
                    style={{ color: (s?.errRate ?? 0) > 5 ? "var(--bad)" : "var(--ink-2)" }}
                  >
                    {s ? `${s.errRate.toFixed(1)}%` : "—"}
                  </TableCell>
                  <TableCell className="text-right cn-mono">{s ? fmtNum(s.calls) : "—"}</TableCell>
                  <TableCell className="text-right cn-mono" style={{ fontWeight: 560 }}>
                    {fmt(s?.spend ?? 0)}
                  </TableCell>
                </TableRow>
              )
            })}
            <TableState
              colSpan={7}
              loading={providers.loading}
              empty={rows.length === 0}
              title={q || kind || state ? "没有匹配的供应商" : "还没有接入供应商"}
              desc={
                q || kind || state
                  ? "换个关键词或清掉筛选条件再看看。"
                  : "接入第一个上游账号后，网关才能把请求转发出去。"
              }
            />
          </TableBody>
        </Table>
      </Card>

      <CreateProviderModal
        open={creating}
        onClose={() => setCreating(false)}
        onDone={() => {
          providers.refetch()
          health.refetch()
        }}
      />
    </div>
  )
}

function CreateProviderModal({
  open,
  onClose,
  onDone,
}: {
  open: boolean
  onClose: () => void
  onDone: () => void
}) {
  const [name, setName] = useState("")
  const [kind, setKind] = useState(KINDS[0].value)
  const [baseUrl, setBaseUrl] = useState("")
  const [credentials, setCredentials] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  const fields = CREDENTIAL_FIELDS[kind] ?? []
  const complete = name.trim() !== "" && fields.every((f) => f.optional || (credentials[f.key] ?? "").trim() !== "")

  const submit = async () => {
    if (!complete) {
      toast.error("请填写名称和该供应商必需的凭证")
      return
    }
    setBusy(true)
    try {
      const config: Record<string, string> = {}
      if (baseUrl.trim()) config.base_url = baseUrl.trim()
      for (const f of fields) {
        const v = (credentials[f.key] ?? "").trim()
        if (v) config[f.key] = v
      }
      await api.post("/api/providers", { Name: name, Kind: kind, Config: config, Status: "active" })
      toast.success("已接入供应商")
      setName("")
      setBaseUrl("")
      setCredentials({})
      onDone()
      onClose()
    } catch (err) {
      // The server usually has a specific reason -- an unsupported kind,
      // a validation failure -- and "请检查凭证" sends the admin to look
      // at the one field that was fine.
      toast.error(tError(err, "接入失败，请检查凭证"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="接入供应商"
      sub="凭证保存后只回显掩码，不再明文返回"
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button tone="primary" disabled={busy} onClick={() => void submit()}>
            {busy ? "接入中…" : "接入"}
          </Button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="名称" optional="必填" hint="团队内部叫法，例如「OpenAI 主账号」。">
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="类型" optional="必填">
          <Select
            variant="input"
            label="类型"
            value={kind}
            onValue={setKind}
            options={KINDS.map((k) => ({
              ...k,
              icon: <Brand kind={k.value} size={14} />,
            }))}
          />
        </Field>
        <Field label="Base URL" hint="留空则使用该厂商的默认地址；自建或代理时填这里。">
          <Input
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
            placeholder={BASE_URL_PLACEHOLDER[kind] ?? ""}
          />
        </Field>
        {fields.map((f) => (
          <Field
            key={f.key}
            label={f.label}
            optional={f.optional ? undefined : "必填"}
            hint={f.hint ?? (f.secret ? "只写不读：保存后无法再取回明文。" : undefined)}
          >
            <Input
              type={f.secret ? "password" : "text"}
              value={credentials[f.key] ?? ""}
              onChange={(e) => setCredentials((c) => ({ ...c, [f.key]: e.target.value }))}
              placeholder={f.placeholder ?? ""}
            />
          </Field>
        ))}
      </div>
    </Modal>
  )
}
