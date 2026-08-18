import { useId, type ReactNode } from "react"
import { Icon, type IconName } from "@/components/console/icon"
import { TableCell, TableRow } from "@/components/console/display"
import { areaPath, linePath } from "@/lib/chart"

// The page-level building blocks, one for one with the hi-fi design's
// shell.jsx helpers. Seven page types share this set across the whole
// console -- no page gets its own bespoke chrome (DESIGN.md 6.3).

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

// Re-exported so the pages keep importing their controls from one place.
// The control itself is no longer a native <select>: that one handed its
// popup to the OS, which drew it differently on every desktop and was the
// last piece of the console the design system could not reach.
export { Select, type SelectOption } from "@/components/console/select"
export { Check, Input, Switch, Textarea } from "@/components/console/form"
export { Card, Skeleton, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Tag, type Tone } from "@/components/console/display"
export { Modal } from "@/components/console/modal"

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
    <TableRow className="hover:bg-transparent">
      <TableCell colSpan={colSpan} className="p-0">
        {loading ? <Loading /> : <Empty title={title} desc={desc} />}
      </TableCell>
    </TableRow>
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

export function Spark({ values, color = "#6366f1" }: { values: number[]; color?: string }) {
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
        stroke="#6366f1"
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
