import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/console/button"
import { Icon } from "@/components/console/icon"
import { Card, Field, Input, Modal, PageHead, Switch, Tag } from "@/components/console/ui"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import type { NotifyChannel } from "@/lib/types"

// 短信 / 邮件配置 -- same pluggable-adapter card variant as 身份源. These
// are the channels the local-account OTP goes out on, which is why an
// unconfigured channel means phone/email login simply cannot work.

const CHANNELS = [
  {
    kind: "sms" as const,
    name: "短信通道",
    icon: "smartphone",
    provider: "aliyun_sms",
    providerLabel: "阿里云短信",
    // `required` mirrors notify.requiredConfig on the server, which is
    // what decides whether a channel may be enabled at all. Keeping the
    // two in step is what stops 保存并启用 from failing after the fact.
    fields: [
      { key: "access_key_id", label: "AccessKey ID", secret: false, required: true },
      { key: "access_key_secret", label: "AccessKey Secret", secret: true, required: true },
      { key: "sign_name", label: "签名", secret: false, required: true },
      { key: "template_code", label: "模板 Code", secret: false, required: true },
      { key: "template_param_key", label: "模板参数名", secret: false, required: false },
    ],
  },
  {
    kind: "email" as const,
    name: "邮件通道",
    icon: "mail",
    provider: "smtp",
    providerLabel: "SMTP",
    // Credentials are deliberately not required: plenty of internal
    // relays accept unauthenticated submission from inside the network.
    fields: [
      { key: "host", label: "SMTP 主机", secret: false, required: true },
      { key: "port", label: "端口", secret: false, required: true },
      { key: "username", label: "用户名", secret: false, required: false },
      { key: "password", label: "密码", secret: true, required: false },
      { key: "from_address", label: "发件地址", secret: false, required: true },
      { key: "from_name", label: "发件人名称", secret: false, required: false },
    ],
  },
]

export function NotifyChannelsPage() {
  return (
    <div className="cn-page">
      <PageHead title="短信 / 邮件配置" sub="发信通道可插拔，本地账号的注册验证码就从这里发出" />

      <div className="cn-grid cn-grid-auto">
        {CHANNELS.map((c) => (
          <ChannelCard key={c.kind} spec={c} />
        ))}
      </div>

      <Card flush={false}>
        <div className="cn-notice">
          <Icon name="shield-alert" size={14} />
          <span>
            凭证保存后只回显掩码，不再明文返回。更换凭证需要重新填写完整值——
            这条对齐后端「密钥只写不读」的存储策略。
          </span>
        </div>
      </Card>
    </div>
  )
}

type ChannelSpec = (typeof CHANNELS)[number]

function ChannelCard({ spec }: { spec: ChannelSpec }) {
  const { member } = useAuth()
  const [channel, setChannel] = useState<NotifyChannel | null>(null)
  const [editing, setEditing] = useState(false)
  const [values, setValues] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const [testing, setTesting] = useState(false)
  const [recipient, setRecipient] = useState("")
  const [sending, setSending] = useState(false)

  // The endpoint answers with { channel, sentThisMonth }, not a bare
  // channel. Reading `.Config` off the wrapper gave undefined every time,
  // so every field showed "—" and `configured` was permanently false --
  // which left the enable switch disabled and the channel impossible to
  // turn on from this page at all.
  const load = () => {
    api
      .get<{ channel: NotifyChannel; sentThisMonth: number }>(`/api/notify-channels/${spec.kind}`)
      .then((res) => {
        setChannel(res.channel)
        // Secrets come back masked, so they are deliberately not seeded
        // into the form: an untouched field means "keep what is stored".
        const seed: Record<string, string> = {}
        for (const f of spec.fields) {
          const v = res.channel?.Config?.[f.key]
          if (!f.secret && v != null) seed[f.key] = String(v)
        }
        setValues(seed)
      })
      .catch(() => setChannel(null))
  }
  useEffect(load, [spec.kind])

  const save = async (enabled: boolean, config = values) => {
    setBusy(true)
    try {
      await api.put(`/api/notify-channels/${spec.kind}`, {
        Kind: spec.kind,
        Provider: spec.provider,
        Config: config,
        Enabled: enabled,
      })
      setEditing(false)
      load()
      toast.success("已保存")
    } catch {
      toast.error("保存失败")
    } finally {
      setBusy(false)
    }
  }

  // Matches the server's own rule rather than "has any value at all":
  // that looser check let the switch look available on a config the
  // backend would refuse to enable.
  const configured = spec.fields
    .filter((f) => f.required)
    .every((f) => String(channel?.Config?.[f.key] ?? "").trim() !== "")

  const sendTest = async () => {
    setSending(true)
    try {
      await api.post(`/api/notify-channels/${spec.kind}/test`, { recipient: recipient.trim() })
      setTesting(false)
      toast.success(`测试邮件已发往 ${recipient.trim()}`)
    } catch (err) {
      // The relay's own words, not a generic failure: "认证失败" and
      // "连接超时" need different fixes and this is the only place that
      // knows which happened.
      toast.error(err instanceof Error && err.message ? `发送失败：${err.message}` : "发送失败")
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="cn-card cn-span-2">
      <div className="cn-card-head">
        <span className="cn-cfg-logo" style={{ width: 28, height: 28, borderRadius: 9 }}>
          <Icon name={spec.icon} size={15} />
        </span>
        <span className="cn-card-title">{spec.name}</span>
        <Tag tone={channel?.Enabled ? "ok" : "warn"}>{channel?.Enabled ? "已启用" : "未启用"}</Tag>
        <span style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          {/* Email only for now -- there is no test path behind the SMS
              channel yet, and a button that cannot do anything is worse
              than one that is not there. */}
          {spec.kind === "email" && (
            <Button
              disabled={!configured}
              onClick={() => {
                setRecipient((v) => v || member?.Email || "")
                setTesting(true)
              }}
            >
              <Icon name="send" size={14} />
              测试
            </Button>
          )}
          <Button onClick={() => setEditing(true)}>
            <Icon name="edit" size={14} />
            配置
          </Button>
          <Switch
            on={!!channel?.Enabled}
            label={`启用${spec.name}`}
            disabled={!configured}
            onToggle={() => void save(!channel?.Enabled, channel?.Config ?? {})}
          />
        </span>
      </div>
      <div className="cn-card-body cn-flush">
        <div className="cn-kv">
          <span className="cn-kv-k">服务商</span>
          <span className="cn-kv-v" style={{ fontFamily: "inherit" }}>
            {spec.providerLabel}
          </span>
        </div>
        {spec.fields.map((f) => (
          <div key={f.key} className="cn-kv">
            <span className="cn-kv-k">{f.label}</span>
            <span className="cn-kv-v">
              {f.secret
                ? channel?.Config?.[f.key]
                  ? "••••••••"
                  : "—"
                : channel?.Config?.[f.key] || "—"}
            </span>
          </div>
        ))}
      </div>

      <Modal
        open={editing}
        title={`配置${spec.name}`}
        sub={spec.providerLabel}
        onClose={() => setEditing(false)}
        footer={
          <>
            <Button disabled={busy} onClick={() => void save(false)}>
              保存但不启用
            </Button>
            <Button tone="primary" disabled={busy} onClick={() => void save(true)}>
              保存并启用
            </Button>
          </>
        }
      >
        <div className="cn-form">
          {spec.fields.map((f) => (
            <Field key={f.key} label={f.label} optional={f.required ? "必填" : undefined}>
              <Input
                type={f.secret ? "password" : "text"}
                value={values[f.key] ?? ""}
                onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                placeholder={f.secret && channel?.Config?.[f.key] ? "已保存，留空表示不修改" : ""}
              />
            </Field>
          ))}
        </div>
      </Modal>

      <Modal
        open={testing}
        title="发送测试邮件"
        sub="用已保存的配置真发一封，验证凭证是否可用"
        onClose={() => setTesting(false)}
        footer={
          <>
            <Button disabled={sending} onClick={() => setTesting(false)}>
              取消
            </Button>
            <Button
              tone="primary"
              disabled={sending || !recipient.trim()}
              onClick={() => void sendTest()}
            >
              {sending ? "发送中…" : "发送"}
            </Button>
          </>
        }
      >
        <div className="cn-form">
          <Field label="收件地址" hint="填你自己能收到的地址。这是一封真实邮件，会记入发信统计。">
            <Input
              type="email"
              value={recipient}
              onChange={(e) => setRecipient(e.target.value)}
              placeholder="you@example.com"
            />
          </Field>
        </div>
      </Modal>
    </div>
  )
}
