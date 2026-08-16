import type { ComponentProps, CSSProperties, ReactNode } from "react"
import { Badge as ShadcnBadge } from "@/components/ui/badge"
import { Card as ShadcnCard, CardContent } from "@/components/ui/card"
import { Skeleton as ShadcnSkeleton } from "@/components/ui/skeleton"
import {
  Table as ShadcnTable,
  TableBody as ShadcnTableBody,
  TableCell as ShadcnTableCell,
  TableHead as ShadcnTableHead,
  TableHeader as ShadcnTableHeader,
  TableRow as ShadcnTableRow,
} from "@/components/ui/table"
import { Icon } from "@/components/console/icon"
import { cn } from "@/lib/utils"

// Console dressing over shadcn's display primitives.
//
// One structural note on Card: only the root and the body come from
// shadcn. Its CardHeader is a container-query grid built for a title /
// description / action layout, while `.cn-card-head` is a plain flex row
// whose third slot is a link button. Bending the grid into that shape
// costs more than it returns, so the head stays the console's own markup
// inside a shadcn Card.

export type Tone = "ok" | "warn" | "bad" | "brand"

const TONE_BG: Record<Tone, string> = {
  ok: "bg-[var(--ok-soft)] text-[var(--ok)]",
  warn: "bg-[var(--warn-soft)] text-[var(--warn)]",
  bad: "bg-[var(--bad-soft)] text-[var(--bad)]",
  brand: "bg-[var(--brand-soft)] text-[var(--brand)]",
}
const TONE_DOT: Record<Tone, string> = {
  ok: "bg-[var(--ok)]",
  warn: "bg-[var(--warn)]",
  bad: "bg-[var(--bad)]",
  brand: "bg-[var(--brand)]",
}

// .cn-tag -- a 20px pill with a 5px status dot.
export function Tag({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <ShadcnBadge
      data-t={tone}
      className={cn(
        // nowrap, not Badge's overridden `whitespace-normal`: squeezed into
        // a narrow status column the pill broke its own label one character
        // per line ("正/常") inside a 20px-tall capsule. It keeps its width
        // and the table scrolls instead.
        "h-[20px] gap-[5px] rounded-full border-0 px-[8px] py-0 text-[10.5px] font-[620] whitespace-nowrap",
        TONE_BG[tone],
      )}
    >
      <i className={cn("size-[5px] rounded-full", TONE_DOT[tone])} />
      {children}
    </ShadcnBadge>
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
    <ShadcnCard
      className={cn(
        // .cn-card: 11px radius, the console's two-layer shadow, and no
        // vertical padding or gap -- shadcn's card is py-6 gap-6.
        "gap-0 overflow-hidden rounded-[11px] border-[var(--line)] bg-[var(--panel)] py-0",
        "shadow-[0_1px_2px_rgba(22,35,58,.05),0_4px_14px_-6px_rgba(22,35,58,.10)]",
        "min-w-0",
        className,
      )}
      style={style}
    >
      {head ??
        ((title || link) && (
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
      <CardContent className={flush ? "px-0" : "p-[15px]"}>{children}</CardContent>
    </ShadcnCard>
  )
}

// .cn-table. shadcn wraps its table in an overflow container; the console
// puts tables inside .cn-card, which already clips, so the wrapper is
// flattened to keep the DOM (and the border-radius clipping) unchanged.
export function Table({ className, ...props }: ComponentProps<typeof ShadcnTable>) {
  return <ShadcnTable className={cn("w-full text-[12.5px]", className)} {...props} />
}

export function TableHeader({ className, ...props }: ComponentProps<typeof ShadcnTableHeader>) {
  return <ShadcnTableHeader className={cn("[&_tr]:border-0", className)} {...props} />
}

export function TableBody({ className, ...props }: ComponentProps<typeof ShadcnTableBody>) {
  return <ShadcnTableBody className={className} {...props} />
}

export function TableRow({ className, ...props }: ComponentProps<typeof ShadcnTableRow>) {
  return (
    <ShadcnTableRow
      className={cn("border-0 hover:bg-[#fafbfd] data-[state=selected]:bg-[#fafbfd]", className)}
      {...props}
    />
  )
}

export function TableHead({ className, ...props }: ComponentProps<typeof ShadcnTableHead>) {
  return (
    <ShadcnTableHead
      className={cn(
        "h-auto bg-[#fafbfd] px-[15px] py-[9px] text-left align-middle",
        "text-[11px] font-[620] tracking-[.02em] whitespace-nowrap text-[var(--ink-3)]",
        "border-b border-[var(--line-2)]",
        className,
      )}
      {...props}
    />
  )
}

export function TableCell({ className, ...props }: ComponentProps<typeof ShadcnTableCell>) {
  return (
    <ShadcnTableCell
      className={cn(
        "px-[15px] py-[10px] align-middle border-b border-[var(--line-2)]",
        "[tr:last-child_&]:border-b-0",
        className,
      )}
      {...props}
    />
  )
}

// .cn-skel -- a sweeping gradient, not shadcn's pulse. The gradient has
// to be restated as utilities rather than by reusing the `.cn-skel`
// class: that rule lives in the `fluxa` layer, which loses to shadcn's
// own `animate-pulse` / `bg-accent` in the utilities layer. The keyframes
// are still app.css's, since @keyframes are not layered.
export function Skeleton({ className, ...props }: ComponentProps<typeof ShadcnSkeleton>) {
  return (
    <ShadcnSkeleton
      className={cn(
        "rounded-[6px] animate-[cnSkel_1.3s_ease_infinite]",
        // The arbitrary-property form is deliberate: a `bg-transparent`
        // would merge away against the gradient, and shadcn's `bg-accent`
        // otherwise stays underneath.
        "bg-[linear-gradient(90deg,#eef1f6_25%,#f6f8fc_37%,#eef1f6_63%)] bg-[length:400%_100%] [background-color:transparent]!",
        className,
      )}
      {...props}
    />
  )
}
