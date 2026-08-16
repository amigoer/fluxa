import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Icon } from "@/components/console/icon"
import { Card, Field, Modal, PageHead, Switch, Tag } from "@/components/console/ui"
import { api } from "@/lib/api"
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
    fields: [
      { key: "access_key_id", label: "AccessKey ID", secret: false },
      { key: "access_key_secret", label: "AccessKey Secret", secret: true },
      { key: "sign_name", label: "签名", secret: false },
      { key: "template_code", label: "模板 Code", secret: false },
      { key: "template_param_key", label: "模板参数名", secret: false },
    ],
  },
  {
    kind: "email" as const,
    name: "邮件通道",
    icon: "mail",
    provider: "smtp",
    providerLabel: "SMTP",
    fields: [
      { key: "host", label: "SMTP 主机", secret: false },
      { key: "port", label: "端口", secret: false },
      { key: "username", label: "用户名", secret: false },
      { key: "password", label: "密码", secret: true },
      { key: "from_address", label: "发件地址", secret: false },
      { key: "from_name", label: "发件人名称", secret: false },
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
  const [channel, setChannel] = useState<NotifyChannel | null>(null)
  const [editing, setEditing] = useState(false)
  const [values, setValues] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  const load = () => {
    api
      .get<NotifyChannel>(`/api/notify-channels/${spec.kind}`)
      .then((c) => {
        setChannel(c)
        setValues(c?.Config ?? {})
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

  const configured = Object.values(channel?.Config ?? {}).some(Boolean)

  return (
    <div className="cn-card cn-span-2">
      <div className="cn-card-head">
        <span className="cn-cfg-logo" style={{ width: 28, height: 28, borderRadius: 9 }}>
          <Icon name={spec.icon} size={15} />
        </span>
        <span className="cn-card-title">{spec.name}</span>
        <Tag tone={channel?.Enabled ? "ok" : "warn"}>{channel?.Enabled ? "已启用" : "未启用"}</Tag>
        <span style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          <button className="cn-btn" onClick={() => setEditing(true)}>
            <Icon name="edit" size={14} />
            配置
          </button>
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
            <button className="cn-btn" disabled={busy} onClick={() => void save(false)}>
              保存但不启用
            </button>
            <button className="cn-btn cn-btn-pri" disabled={busy} onClick={() => void save(true)}>
              保存并启用
            </button>
          </>
        }
      >
        <div className="cn-form">
          {spec.fields.map((f) => (
            <Field key={f.key} label={f.label}>
              <input
                className="cn-input"
                type={f.secret ? "password" : "text"}
                value={values[f.key] ?? ""}
                onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                placeholder={f.secret && channel?.Config?.[f.key] ? "已保存，留空表示不修改" : ""}
              />
            </Field>
          ))}
        </div>
      </Modal>
    </div>
  )
}
