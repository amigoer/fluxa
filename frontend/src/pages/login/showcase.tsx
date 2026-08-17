import { useEffect, useState } from "react"
import { Icon } from "@/components/console/icon"

// The panel beside the sign-in column. The reference layout fills this
// half with a photograph of the product in use; there is no honest
// equivalent for a gateway that runs in a customer's own machine room,
// and a stock photo of an office would say nothing true. So it carries
// what this deployment actually does instead -- the four things from
// DESIGN.md 2.1 that an employee reaching this screen is about to be
// governed by -- drawn in the console's own tokens rather than imagery.
const SLIDES = [
  {
    icon: "wallet",
    title: "统一采购，按额度分发",
    body: "企业集中采购模型额度，再按部门和成员分发。入库、分发、消耗记在同一本账上，随时对得起来。",
  },
  {
    icon: "shield-alert",
    title: "敏感信息不出企业",
    body: "身份证号、银行卡号、手机号在请求发往供应商之前识别并脱敏，命中即留痕，不依赖员工自觉。",
  },
  {
    icon: "waypoints",
    title: "路由与熔断由你定义",
    body: "按条件挑目标模型，失败沿 fallback 链回退；供应商连续异常自动熔断隔离，冷却后自动试探恢复。",
  },
  {
    icon: "scroll-text",
    title: "每一次调用都有据可查",
    body: "谁、什么时候、用了哪个模型、花了多少，逐条留存；管理动作另有一份不可篡改的操作审计。",
  },
]

// Long enough to finish reading the body text, which is the only reason
// the panel rotates at all.
const DWELL_MS = 6000

export function AuthShowcase() {
  const [at, setAt] = useState(0)

  useEffect(() => {
    // Somebody who has asked for less motion gets the first panel and no
    // movement; the content is not sequential, so nothing is lost.
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return
    const t = setInterval(() => setAt((i) => (i + 1) % SLIDES.length), DWELL_MS)
    return () => clearInterval(t)
  }, [])

  return (
    <aside className="cn-auth-side" aria-label="Fluxa 能做什么">
      <div className="cn-auth-side-inner">
        {/* The reference layout fills this half with a photograph. With
            none to use, an oversized draw of the current slide's own
            glyph carries the upper two thirds -- it changes with the
            slide, so the panel reads as one composition rather than a
            gradient with text parked at the bottom. */}
        <div className="cn-auth-mark" aria-hidden="true">
          {SLIDES.map((s, i) => (
            <span key={s.title} data-on={i === at}>
              <Icon name={s.icon} size={260} stroke={0.55} />
            </span>
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

        <div className="cn-auth-dots">
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
