import { useMemo, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Field, Filters, Input, Modal, PageHead, Select, Switch, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableState, Tag } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import type { DLPRule, SecurityEvent } from "@/lib/types"

// DLP 规则 -- list page. Rules are matched top-down by priority against
// the request body: "拦截" ends the request, "脱敏" substitutes and lets
// it through.
export function DlpRulesPage() {
  const rules = useApiQuery<DLPRule[]>("/api/dlp-rules")
  const { data: events } = useApiQuery<SecurityEvent[]>("/api/security-events")

  const [q, setQ] = useState("")
  const [action, setAction] = useState("")
  const [enabled, setEnabled] = useState("")
  const [creating, setCreating] = useState(false)

  // 今日命中 comes from the event log rather than a counter on the rule --
  // same source the security-events page reads, so the two never disagree.
  const hitsToday = useMemo(() => {
    const start = new Date()
    start.setHours(0, 0, 0, 0)
    const out = new Map<string, number>()
    for (const e of events ?? []) {
      if (!e.RuleID || new Date(e.OccurredAt) < start) continue
      out.set(e.RuleID, (out.get(e.RuleID) ?? 0) + 1)
    }
    return out
  }, [events])

  const rows = (rules.data ?? [])
    .filter((r) => {
      if (q && !`${r.Name}${r.Pattern}`.toLowerCase().includes(q.toLowerCase())) return false
      if (action && r.Action !== action) return false
      if (enabled && String(r.Enabled) !== enabled) return false
      return true
    })
    .sort((a, b) => a.Priority - b.Priority)

  const toggle = async (rule: DLPRule) => {
    try {
      await api.patch(`/api/dlp-rules/${rule.ID}/enabled`, { enabled: !rule.Enabled })
      rules.refetch()
    } catch {
      toast.error("操作失败")
    }
  }

  const remove = async (rule: DLPRule) => {
    if (!confirm(`删除规则「${rule.Name}」？此后命中这条模式的请求不再被处理。`)) return
    try {
      await api.delete(`/api/dlp-rules/${rule.ID}`)
      toast.success("已删除")
      rules.refetch()
    } catch {
      toast.error("删除失败")
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="DLP 规则" sub="按优先级自上而下匹配请求体；「拦截」直接终止请求，「脱敏」替换后放行">
        <Button tone="primary" onClick={() => setCreating(true)}>
          <Icon name="plus" size={14} />
          新增规则
        </Button>
      </PageHead>

      <Filters
        placeholder="搜索规则名或模式…"
        value={q}
        onValue={setQ}
        right={
          <span className="cn-count">
            {(rules.data ?? []).filter((r) => r.Enabled).length} 条启用 · {(rules.data ?? []).length} 条总计
          </span>
        }
      >
        <Select
          label="动作"
          value={action}
          onValue={setAction}
          options={[
            { value: "", label: "全部动作" },
            { value: "block", label: "拦截" },
            { value: "mask", label: "脱敏" },
          ]}
        />
        <Select
          label="状态"
          value={enabled}
          onValue={setEnabled}
          options={[
            { value: "", label: "全部状态" },
            { value: "true", label: "已启用" },
            { value: "false", label: "已停用" },
          ]}
        />
      </Filters>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="text-right" style={{ width: 60 }}>
                优先级
              </TableHead>
              <TableHead>规则名</TableHead>
              <TableHead>匹配方式</TableHead>
              <TableHead>模式</TableHead>
              <TableHead>动作</TableHead>
              <TableHead className="text-right">今日命中</TableHead>
              <TableHead className="text-right">启用</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.ID} style={r.Enabled ? undefined : { opacity: 0.55 }}>
                <TableCell className="text-right cn-mono" style={{ color: "var(--ink-3)" }}>
                  {r.Priority}
                </TableCell>
                <TableCell style={{ fontWeight: 570 }}>{r.Name}</TableCell>
                <TableCell style={{ color: "var(--ink-2)" }}>{r.MatchType === "keyword" ? "关键词" : "正则 + 校验位"}</TableCell>
                <TableCell>
                  <span className="cn-mono-cell cn-trunc">{r.Pattern}</span>
                </TableCell>
                <TableCell>
                  <Tag tone={r.Action === "block" ? "bad" : "warn"}>{r.Action === "block" ? "拦截" : "脱敏"}</Tag>
                </TableCell>
                <TableCell className="text-right cn-mono" style={{ fontWeight: (hitsToday.get(r.ID) ?? 0) > 20 ? 600 : 400 }}>
                  {hitsToday.get(r.ID) || "—"}
                </TableCell>
                <TableCell className="text-right">
                  <Switch
                    on={r.Enabled}
                    label={`启用 ${r.Name}`}
                    onToggle={() => void toggle(r)}
                    style={{ display: "inline-block", verticalAlign: "middle" }}
                  />
                </TableCell>
                <TableCell>
                  <div className="cn-row-acts">
                    <Button tone="icon" data-danger="true" title="删除" onClick={() => void remove(r)}>
                      <Icon name="trash" size={14} />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            <TableState
              colSpan={8}
              loading={rules.loading}
              empty={rows.length === 0}
              title="还没有 DLP 规则"
              desc="没有规则时，请求体原样转发给上游供应商。"
            />
          </TableBody>
        </Table>
      </Card>

      <NewRuleModal
        open={creating}
        nextPriority={Math.max(0, ...(rules.data ?? []).map((r) => r.Priority)) + 10}
        onClose={() => setCreating(false)}
        onDone={() => rules.refetch()}
      />
    </div>
  )
}

function NewRuleModal({
  open,
  nextPriority,
  onClose,
  onDone,
}: {
  open: boolean
  nextPriority: number
  onClose: () => void
  onDone: () => void
}) {
  const [name, setName] = useState("")
  const [matchType, setMatchType] = useState<"regex_checksum" | "keyword">("keyword")
  const [pattern, setPattern] = useState("")
  const [action, setAction] = useState<"mask" | "block">("mask")
  const [priority, setPriority] = useState(String(nextPriority))
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!name || !pattern) {
      toast.error("请填写规则名和模式")
      return
    }
    setBusy(true)
    try {
      await api.post("/api/dlp-rules", {
        Name: name,
        MatchType: matchType,
        Pattern: pattern,
        Action: action,
        Priority: Number(priority) || 0,
        Enabled: true,
      })
      toast.success("已新增规则")
      setName("")
      setPattern("")
      onDone()
      onClose()
    } catch {
      toast.error("新增失败，请检查模式是否合法")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="新增 DLP 规则"
      sub="规则按优先级从小到大匹配，命中即按其动作处理"
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button tone="primary" disabled={busy} onClick={() => void submit()}>
            {busy ? "保存中…" : "保存并启用"}
          </Button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="规则名" optional="必填">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="手机号" />
        </Field>
        <Field label="匹配方式" optional="必填" hint="正则 + 校验位适合身份证、银行卡这类有校验规则的号码。">
          <Select
            variant="input"
            label="匹配方式"
            value={matchType}
            onValue={(v) => setMatchType(v as "regex_checksum" | "keyword")}
            options={[
              { value: "keyword", label: "关键词" },
              { value: "regex_checksum", label: "正则 + 校验位" },
            ]}
          />
        </Field>
        <Field label="模式" optional="必填">
          <Input
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            placeholder={matchType === "keyword" ? "内部仓库地址" : "1[3-9]\\d{9}"}
          />
        </Field>
        <Field label="动作" optional="必填" hint="拦截会直接终止请求并记录事件；脱敏替换后继续转发。">
          <Select
            variant="input"
            label="动作"
            value={action}
            onValue={(v) => setAction(v as "mask" | "block")}
            options={[
              { value: "mask", label: "脱敏" },
              { value: "block", label: "拦截" },
            ]}
          />
        </Field>
        <Field label="优先级" hint="数字越小越先匹配。">
          <Input
            inputMode="numeric"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
        </Field>
      </div>
    </Modal>
  )
}
