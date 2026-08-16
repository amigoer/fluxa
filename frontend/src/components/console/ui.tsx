import { useEffect, useId, type CSSProperties, type ReactNode } from "react"
import { Icon, type IconName } from "@/components/console/icon"
import { areaPath, linePath } from "@/lib/chart"

// The page-level building blocks, one for one with the hi-fi design's
// shell.jsx helpers. Seven page types share this set across the whole
// console -- no page gets its own bespoke chrome (DESIGN.md 6.3).

export type Tone = "ok" | "warn" | "bad" | "brand"

export function PageHead({ title, sub, children }: { title: string; sub?: ReactNode; children?: ReactNode }) {
  return (
    <div className="cn-page-head">
      <div>
        <h1 className="cn-page-title">{title}</h1>
        {sub && <p className="cn-page-sub">{sub}</p>}
      </div>
      {children && <div className="cn-page-acts">{children}</div>}
    </div>
  )
}

export function Card({
  title,
  note,
  link,
  onLink,
  children,
  flush = true,
  className = "",
  style,
  head,
}: {
  title?: ReactNode
  note?: ReactNode
  link?: ReactNode
  onLink?: () => void
  children?: ReactNode
  flush?: boolean
  className?: string
  style?: CSSProperties
  head?: ReactNode
}) {
  return (
    <div className={`cn-card ${className}`} style={style}>
      {head ?? ((title || link) && (
        <div className="cn-card-head">
          {title && <span className="cn-card-title">{title}</span>}
          {note && <span className="cn-card-note">{note}</span>}
          {link && (
            <button className="cn-card-link" onClick={onLink}>
              {link} <Icon name="chevron-right" size={12} />
            </button>
          )}
        </div>
      ))}
      <div className={flush ? "cn-card-body cn-flush" : "cn-card-body"}>{children}</div>
    </div>
  )
}

export function Tag({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className="cn-tag" data-t={tone}>
      <i />
      {children}
    </span>
  )
}

// The filter row: a search box plus any number of selects. The mockup
// draws these as divs; here they are real controls, which is the only
// difference between the two.
export function Filters({
  placeholder = "搜索…",
  value,
  onValue,
  children,
  right,
}: {
  placeholder?: string
  value?: string
  onValue?: (v: string) => void
  children?: ReactNode
  right?: ReactNode
}) {
  return (
    <div className="cn-filters">
      <label className="cn-field cn-field-search">
        <Icon name="search" size={14} />
        <input
          value={value ?? ""}
          onChange={(e) => onValue?.(e.target.value)}
          placeholder={placeholder}
          aria-label={placeholder}
        />
      </label>
      {children}
      {right && <div className="cn-filters-right">{right}</div>}
    </div>
  )
}

export function Select({
  value,
  onValue,
  options,
  label,
}: {
  value: string
  onValue: (v: string) => void
  options: { value: string; label: string }[]
  label?: string
}) {
  return (
    <select
      className="cn-field cn-field-select"
      value={value}
      aria-label={label}
      onChange={(e) => onValue(e.target.value)}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}

export function Switch({
  on,
  onToggle,
  disabled,
  label,
  style,
}: {
  on: boolean
  onToggle?: () => void
  disabled?: boolean
  label?: string
  style?: CSSProperties
}) {
  return (
    <button
      type="button"
      className="cn-switch"
      data-on={on}
      role="switch"
      aria-checked={on}
      aria-label={label}
      disabled={disabled || !onToggle}
      style={style}
      onClick={onToggle}
    />
  )
}

export function Check({
  on,
  locked,
  onToggle,
  label,
}: {
  on: boolean
  locked?: boolean
  onToggle?: () => void
  label?: string
}) {
  const Tag = onToggle ? "button" : "span"
  return (
    <Tag
      className="cn-check"
      data-on={on}
      data-locked={!!(on && locked)}
      role="checkbox"
      aria-checked={on}
      aria-label={label}
      onClick={onToggle}
      {...(onToggle ? { type: "button" as const } : {})}
    >
      {on && <Icon name="check" size={11} stroke={2.6} />}
    </Tag>
  )
}

export function Empty({
  icon = "inbox",
  title,
  desc,
  children,
}: {
  icon?: IconName | string
  title: string
  desc?: string
  children?: ReactNode
}) {
  return (
    <div className="cn-empty">
      <div className="cn-empty-ico">
        <Icon name={icon} size={19} />
      </div>
      <b>{title}</b>
      {desc && <p>{desc}</p>}
      {children}
    </div>
  )
}

export function Loading({ label = "加载中…" }: { label?: string }) {
  return <div className="cn-loading">{label}</div>
}

// A table body that is empty for a reason: still loading, or genuinely
// nothing there. Both need to keep the header row visible, so they render
// as a full-width cell rather than replacing the table.
export function TableState({
  colSpan,
  loading,
  empty,
  title,
  desc,
}: {
  colSpan: number
  loading?: boolean
  empty?: boolean
  title: string
  desc?: string
}) {
  if (!loading && !empty) return null
  return (
    <tr>
      <td colSpan={colSpan} style={{ padding: 0 }}>
        {loading ? <Loading /> : <Empty title={title} desc={desc} />}
      </td>
    </tr>
  )
}

// ---- dialogs ---------------------------------------------------------

export function Modal({
  open,
  title,
  sub,
  onClose,
  children,
  footer,
  wide,
}: {
  open: boolean
  title: string
  sub?: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="cn-modal-scrim" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className={wide ? "cn-modal cn-modal-wide" : "cn-modal"} role="dialog" aria-modal="true" aria-label={title}>
        <div className="cn-modal-head">
          <div>
            <div className="cn-modal-title">{title}</div>
            {sub && <div className="cn-modal-sub">{sub}</div>}
          </div>
          <button className="cn-modal-close" onClick={onClose} aria-label="关闭">
            <Icon name="x" size={16} />
          </button>
        </div>
        <div className="cn-modal-body">{children}</div>
        {footer && <div className="cn-modal-foot">{footer}</div>}
      </div>
    </div>
  )
}

export function Field({
  label,
  hint,
  optional,
  children,
}: {
  label: string
  hint?: ReactNode
  optional?: string
  children: ReactNode
}) {
  return (
    <div className="cn-form-row">
      <label className="cn-form-label">
        {label}
        {optional && <span>{optional}</span>}
      </label>
      {children}
      {hint && <div className="cn-input-hint">{hint}</div>}
    </div>
  )
}

// ---- small charts ----------------------------------------------------

export function Spark({ values, color = "#2f5fea" }: { values: number[]; color?: string }) {
  const id = useId()
  const w = 96
  const h = 40
  if (values.length < 2) return null
  return (
    <svg className="cn-kpi-spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <linearGradient id={`cs${id}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity=".28" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={areaPath(values, w, h, 5)} fill={`url(#cs${id})`} />
      <path d={linePath(values, w, h, 5)} fill="none" stroke={color} strokeWidth="1.4" />
    </svg>
  )
}

export function Gauge({ pct, size = 76 }: { pct: number; size?: number }) {
  const r = 30
  const c = 2 * Math.PI * r
  const clamped = Math.max(0, Math.min(100, pct))
  return (
    <svg width={size} height={size} viewBox="0 0 76 76" style={{ flex: "none" }} aria-hidden="true">
      <circle cx="38" cy="38" r={r} fill="none" stroke="#eaeef4" strokeWidth="9" />
      <circle
        cx="38"
        cy="38"
        r={r}
        fill="none"
        stroke="#2f5fea"
        strokeWidth="9"
        strokeLinecap="round"
        strokeDasharray={`${(clamped / 100) * c} ${c}`}
        transform="rotate(-90 38 38)"
      />
      <text
        x="38"
        y="42"
        textAnchor="middle"
        fontSize="15"
        fontWeight="700"
        fill="#16233a"
        style={{ fontVariantNumeric: "tabular-nums" }}
      >
        {Math.round(clamped)}%
      </text>
    </svg>
  )
}
