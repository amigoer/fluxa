import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, Field, Modal, PageHead, Switch, Tag } from "@/components/console/ui"
import { api } from "@/lib/api"
import type { AuthSettings, IdentityConfig } from "@/lib/types"

// 身份源 -- the pluggable-adapter card variant, shared with the notify
// channel page. OAuth credentials live in the database, not in a config
// file, so they can be rotated without a redeploy.
//
// Only Feishu has a login route behind it today (internal/user/identity).
// WeCom and DingTalk are drawn because the design does, but their config
// buttons stay disabled rather than storing credentials nothing reads.
const SOURCES = [
  { key: "feishu", name: "飞书", implemented: true },
  { key: "wecom", name: "企业微信", implemented: false },
  { key: "dingtalk", name: "钉钉", implemented: false },
]

export function IdentitySourcesPage() {
  return (
    <div className="cn-page">
      <PageHead title="身份源" sub="OAuth 凭证填在这里，不写死在配置文件；回调地址需要在对应平台后台登记" />

      <div className="cn-grid cn-grid-auto">
        {SOURCES.map((s) => (
          <IdentityCard key={s.key} sourceKey={s.key} name={s.name} implemented={s.implemented} />
        ))}
      </div>

      <LocalAccountCard />
    </div>
  )
}

function IdentityCard({
  sourceKey,
  name,
  implemented,
}: {
  sourceKey: string
  name: string
  implemented: boolean
}) {
  const [config, setConfig] = useState<IdentityConfig | null>(null)
  const [editing, setEditing] = useState(false)
  const [appId, setAppId] = useState("")
  const [appSecret, setAppSecret] = useState("")
  const [callback, setCallback] = useState(`/api/auth/${sourceKey}/callback`)
  const [busy, setBusy] = useState(false)

  const load = () => {
    api
      .get<IdentityConfig>(`/api/identity-configs/${sourceKey}`)
      .then((c) => {
        setConfig(c)
        if (c?.AppID) setAppId(c.AppID)
        if (c?.CallbackPath) setCallback(c.CallbackPath)
      })
      .catch(() => setConfig(null))
  }
  useEffect(load, [sourceKey])

  const save = async (enabled: boolean, secret = appSecret) => {
    setBusy(true)
    try {
      await api.put(`/api/identity-configs/${sourceKey}`, {
        AppID: appId,
        AppSecret: secret,
        CallbackPath: callback,
        Enabled: enabled,
      })
      setEditing(false)
      setAppSecret("")
      load()
      toast.success("已保存")
    } catch {
      toast.error("保存失败")
    } finally {
      setBusy(false)
    }
  }

  const configured = !!config?.AppID

  return (
    <div className="cn-card cn-span-2">
      <div className="cn-cfg-card">
        <span className="cn-cfg-logo">
          {sourceKey === "wecom" ? (
            <span style={{ fontSize: 13, fontWeight: 700, color: "var(--ink-3)" }}>企</span>
          ) : (
            <Brand kind={sourceKey} size={20} />
          )}
        </span>
        <div className="cn-cfg-main">
          <div className="cn-cfg-name">
            {name}
            {!implemented ? (
              <Tag tone="warn">待实现</Tag>
            ) : config?.Enabled ? (
              <Tag tone="ok">已启用</Tag>
            ) : configured ? (
              <Tag tone="warn">已配置 · 未启用</Tag>
            ) : (
              <Tag tone="warn">未配置</Tag>
            )}
          </div>
          <div className="cn-cfg-sub">
            {config?.AppID || "App ID 未填写"} · {config?.CallbackPath || callback}
          </div>
        </div>
        <div className="cn-cfg-acts">
          <button className="cn-btn" disabled={!implemented} onClick={() => setEditing(true)}>
            <Icon name="edit" size={14} />
            配置
          </button>
          <Switch
            on={!!config?.Enabled}
            label={`启用${name}登录`}
            disabled={!implemented || !configured}
            onToggle={() => void save(!config?.Enabled, "")}
          />
        </div>
      </div>

      <Modal
        open={editing}
        title={`配置${name}`}
        sub="App Secret 保存后只回显掩码，更换需要重新填写完整值"
        onClose={() => setEditing(false)}
        footer={
          <>
            <button className="cn-btn" disabled={busy} onClick={() => void save(false)}>
              保存但不启用
            </button>
            <button className="cn-btn cn-btn-pri" disabled={busy || !appId || !appSecret} onClick={() => void save(true)}>
              保存并启用
            </button>
          </>
        }
      >
        <div className="cn-form">
          <Field label="App ID" optional="必填">
            <input className="cn-input" value={appId} onChange={(e) => setAppId(e.target.value)} />
          </Field>
          <Field label="App Secret" optional="必填" hint="只写不读：保存后无法再取回明文。">
            <input
              className="cn-input"
              type="password"
              value={appSecret}
              onChange={(e) => setAppSecret(e.target.value)}
              placeholder={configured ? "已保存，留空表示不修改" : ""}
            />
          </Field>
          <Field label="回调路径" hint={`需要在${name}开放平台后台登记同样的地址。`}>
            <input className="cn-input" value={callback} onChange={(e) => setCallback(e.target.value)} />
          </Field>
        </div>
      </Modal>
    </div>
  )
}

function LocalAccountCard() {
  const [settings, setSettings] = useState<AuthSettings | null>(null)

  const load = () => {
    api.get<AuthSettings>("/api/auth-settings").then(setSettings).catch(() => setSettings(null))
  }
  useEffect(load, [])

  const update = async (patch: Partial<AuthSettings>) => {
    if (!settings) return
    const next = { ...settings, ...patch }
    setSettings(next)
    try {
      await api.put("/api/auth-settings", next)
    } catch {
      toast.error("保存失败")
      load()
    }
  }

  return (
    <Card title="本地账号" note="没有统一 IM 的兜底方案">
      <div className="cn-cfg-card">
        <span className="cn-cfg-logo">
          <Icon name="smartphone" size={18} />
        </span>
        <div className="cn-cfg-main">
          <div className="cn-cfg-name">开放自注册</div>
          <div className="cn-cfg-sub" style={{ fontFamily: "inherit" }}>
            允许用手机号 / 邮箱注册，验证码从「短信 / 邮件配置」的通道发出
          </div>
        </div>
        <div className="cn-cfg-acts">
          <Switch
            on={!!settings?.LocalAccountEnabled}
            label="开放自注册"
            onToggle={() => void update({ LocalAccountEnabled: !settings?.LocalAccountEnabled })}
          />
        </div>
      </div>
      <div className="cn-cfg-card" style={{ borderTop: "1px solid var(--line-2)" }}>
        <span className="cn-cfg-logo">
          <Icon name="shield-check" size={18} />
        </span>
        <div className="cn-cfg-main">
          <div className="cn-cfg-name">注册后需管理员审批</div>
          <div className="cn-cfg-sub" style={{ fontFamily: "inherit" }}>
            关闭后自注册账号立即可用，不再进入待审核队列
          </div>
        </div>
        <div className="cn-cfg-acts">
          <Switch
            on={!!settings?.LocalAccountRequiresApproval}
            label="注册后需管理员审批"
            onToggle={() => void update({ LocalAccountRequiresApproval: !settings?.LocalAccountRequiresApproval })}
          />
        </div>
      </div>
    </Card>
  )
}
