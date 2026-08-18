import { useEffect, useState, type CSSProperties, type ReactNode } from "react"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"

// The panel beside the sign-in column. The reference layout fills this
// half with lifestyle photography -- a person at a desk, vendor logos
// stuck over the top like stickers, a caption pressed into the bottom
// corner. There is no honest equivalent of the photograph for a gateway
// that runs in a customer's own machine room, and a stock office shot
// would say nothing true (DESIGN.md 6.3), so the frame is drawn in the
// console's own tokens instead.
//
// One drawing per claim. Each slide draws the thing it is actually
// claiming -- the providers themselves, an allocation, a masked field, a
// fallback chain, a ledger row -- rather than all four sharing one standing composition, which
// leaves a picture parked next to an argument it is not making. What
// holds them together is a shared vocabulary rather than shared content:
// white objects with soft shadows and a slight tilt, lying on the indigo
// field.
//
// The names and numbers are illustrative, and deliberately shaped like
// real ones -- a department with a share of a pool, an ID with its middle
// removed, a provider that tripped. None of it is read from the
// deployment; a login screen has no session to read it with.

// Model vendors only. Feishu is the primary button on the other half of
// the screen, so repeating it here would turn the group into a list of
// login methods rather than of what is behind the door.
const PROVIDERS = ["openai", "anthropic", "gemini", "azure_openai", "bedrock", "alibaba"]

const Gateway = () => (
  <>
    {PROVIDERS.map((kind) => (
      <span key={kind} className="cn-auth-float cn-auth-chip">
        <Brand kind={kind} style={{ width: "58%", height: "58%" }} />
      </span>
    ))}
  </>
)

const Quota = () => (
  <>
    {[
      { dept: "研发部", used: "1.24M", pct: 74 },
      { dept: "产品部", used: "480K", pct: 46 },
      { dept: "设计部", used: "210K", pct: 28 },
    ].map((r) => (
      <div key={r.dept} className="cn-auth-float cn-auth-card">
        <div className="cn-auth-card-row">
          <span className="cn-auth-card-val">{r.dept}</span>
          <span className="cn-auth-card-spacer cn-auth-card-mono">{r.used}</span>
        </div>
        <span className="cn-auth-bar">
          <i style={{ width: `${r.pct}%` }} />
        </span>
      </div>
    ))}
  </>
)

const Redaction = () => (
  <>
    {[
      { lab: "手机号", head: "138", tail: "8000", hit: false },
      { lab: "身份证", head: "3301", tail: "1024", hit: true },
      { lab: "银行卡", head: "6222", tail: "9012", hit: false },
    ].map((r) => (
      <div key={r.lab} className="cn-auth-float cn-auth-card">
        <div className="cn-auth-card-row">
          <span className="cn-auth-card-lab">{r.lab}</span>
          <span className="cn-auth-card-mono">
            {r.head}
            <i className="cn-auth-redact" />
            {r.tail}
          </span>
          {r.hit && (
            <span className="cn-auth-card-spacer cn-auth-hit">
              <Icon name="shield-alert" size={11} />
              已脱敏
            </span>
          )}
        </div>
      </div>
    ))}
  </>
)

const Routing = () => (
  <div className="cn-auth-chain">
    {[
      { kind: "openai", name: "gpt-4o", note: "目标模型", tripped: false },
      { kind: "anthropic", name: "claude-sonnet", note: "fallback", tripped: false },
      { kind: "azure_openai", name: "azure-gpt-4o", note: null, tripped: true },
    ].map((n) => (
      <div key={n.kind} className="cn-auth-node" data-tripped={n.tripped}>
        <div className="cn-auth-float cn-auth-card">
          <div className="cn-auth-card-row">
            <span className="cn-auth-node-mark">
              <Brand kind={n.kind} style={{ width: "100%", height: "100%" }} />
            </span>
            <span className="cn-auth-card-val">{n.name}</span>
            {n.tripped ? (
              <span className="cn-auth-card-spacer cn-auth-trip">
                <Icon name="alert-triangle" size={11} />
                已熔断隔离
              </span>
            ) : (
              <span className="cn-auth-card-spacer cn-auth-card-lab">{n.note}</span>
            )}
          </div>
        </div>
      </div>
    ))}
  </div>
)

const Ledger = () => (
  <>
    {[
      { at: "14:02:07", kind: "openai", model: "gpt-4o", cost: "¥0.42" },
      { at: "14:01:55", kind: "anthropic", model: "claude-sonnet", cost: "¥0.31" },
      { at: "14:01:38", kind: "gemini", model: "gemini-2.5-pro", cost: "¥0.18" },
    ].map((r) => (
      <div key={r.at} className="cn-auth-float cn-auth-card">
        <div className="cn-auth-card-row">
          <span className="cn-auth-card-lab cn-auth-card-mono">{r.at}</span>
          <Brand kind={r.kind} style={{ width: "1.5em", height: "1.5em", flex: "none" }} />
          <span className="cn-auth-card-val">{r.model}</span>
          <span className="cn-auth-card-spacer cn-auth-cost">{r.cost}</span>
        </div>
      </div>
    ))}
  </>
)

// What an employee reaching this screen is about to be governed by: the
// four goals from DESIGN.md 2.1, led by the premise the other four sit on
// top of -- that all of it arrives through one endpoint.
const SLIDES: { icon: string; title: string; body: string; art: ReactNode; artClass: string }[] = [
  {
    icon: "server",
    title: "一个入口，接住所有供应商",
    body: "各家供应商的地址和密钥都收在网关里，员工那一侧只有一个 Base URL 和一把 Key，换供应商不用改代码。",
    art: <Gateway />,
    artClass: "cn-auth-art-gate",
  },
  {
    icon: "wallet",
    title: "统一采购，按额度分发",
    body: "企业集中采购模型额度，再按部门和成员分发。入库、分发、消耗记在同一本账上，随时对得起来。",
    art: <Quota />,
    artClass: "cn-auth-art-quota",
  },
  {
    icon: "shield-alert",
    title: "敏感信息不出企业",
    body: "身份证号、银行卡号、手机号在请求发往供应商之前识别并脱敏，命中即留痕，不依赖员工自觉。",
    art: <Redaction />,
    artClass: "cn-auth-art-dlp",
  },
  {
    icon: "waypoints",
    title: "路由与熔断由你定义",
    body: "按条件挑目标模型，失败沿 fallback 链回退；供应商连续异常自动熔断隔离，冷却后自动试探恢复。",
    art: <Routing />,
    artClass: "cn-auth-art-route",
  },
  {
    icon: "scroll-text",
    title: "每一次调用都有据可查",
    body: "谁、什么时候、用了哪个模型、花了多少，逐条留存；管理动作另有一份不可篡改的操作审计。",
    art: <Ledger />,
    artClass: "cn-auth-art-log",
  },
]

// Long enough to finish reading the body text, which is the only reason
// the panel rotates at all.
const DWELL_MS = 6000

export function AuthShowcase() {
  const [at, setAt] = useState(0)

  // A timeout re-armed per slide rather than one standing interval, so a
  // click on a dot restarts the dwell instead of landing mid-way through
  // a tick that is about to move the panel again. The bars below draw
  // this countdown, so a clock that kept running would be visibly wrong.
  useEffect(() => {
    // Somebody who has asked for less motion gets the first panel and no
    // movement; the content is not sequential, so nothing is lost.
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return
    const t = setTimeout(() => setAt((i) => (i + 1) % SLIDES.length), DWELL_MS)
    return () => clearTimeout(t)
  }, [at])

  return (
    <aside className="cn-auth-side" aria-label="Fluxa 能做什么">
      <div className="cn-auth-side-inner">
        {/* Decoration in the strict sense: nothing here is needed to sign
            in, and the figures are illustrative, so none of it is read
            out. */}
        <div className="cn-auth-dust" aria-hidden="true">
          <i /><i /><i /><i /><i /><i />
        </div>
        <div className="cn-auth-arts" aria-hidden="true">
          {SLIDES.map((s, i) => (
            <div key={s.title} className={`cn-auth-art ${s.artClass}`} data-on={i === at}>
              {s.art}
            </div>
          ))}
        </div>

        <div className="cn-auth-slides">
          {SLIDES.map((s, i) => (
            <div key={s.title} className="cn-auth-slide" data-on={i === at} aria-hidden={i !== at}>
              <span className="cn-auth-slide-ico">
                <Icon name={s.icon} size={20} />
              </span>
              <h2 className="cn-auth-slide-title">{s.title}</h2>
              <p className="cn-auth-slide-body">{s.body}</p>
            </div>
          ))}
        </div>

        {/* The dwell is handed to CSS rather than duplicated there: the bars
            animate for exactly as long as the slide is up. */}
        <div className="cn-auth-dots" style={{ "--cn-dwell": `${DWELL_MS}ms` } as CSSProperties}>
          {SLIDES.map((s, i) => (
            <button
              key={s.title}
              className="cn-auth-dot"
              data-on={i === at}
              onClick={() => setAt(i)}
              aria-label={s.title}
            />
          ))}
        </div>
      </div>
    </aside>
  )
}
