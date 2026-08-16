import type { CSSProperties, ReactNode } from "react"
import {
  Select as SelectRoot,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

// The console's own dressing over shadcn's Select. Two jobs: keep the
// `{ value, onValue, options }` signature the 11 pages already call, and
// pull shadcn's 14px/36px default geometry down to the console's own
// (34px filter pill, 12.5px type, the .cn-menu popover shape).
//
// Everything visual here is an arbitrary-value utility rather than a rule
// in console.css on purpose: Tailwind's utilities layer sits after the
// `fluxa` layer, so a stylesheet rule would lose to the component's own
// classes. Overriding in the same layer is what twMerge is for.

export type SelectOption = {
  value: string
  label: string
  /** Leading mark -- <Brand kind={…} /> for anything vendor-specific.
   *  It renders in the trigger too, since Radix mirrors the selected
   *  item's content up into the value slot. */
  icon?: ReactNode
  disabled?: boolean
}

// Radix reserves "" to mean "nothing selected" -- it is precisely what
// makes Select.Value fall back to its placeholder. The console also uses
// "" for the 全部 / 无 / 未分配 rows, which are real selectable items, so
// those are swapped for a sentinel going in and swapped back coming out.
//
// That swap is conditional on such a row actually existing (see
// `emptyIsAnOption` below). Sending the sentinel unconditionally broke
// every select that has no ""-valued option -- a form field with nothing
// chosen yet matched no item at all, so Radix rendered neither the
// selected label nor the placeholder, and set no `data-placeholder` for
// the trigger to style. The result was a blank box where a required
// field should be (Playground 模型, 登记入库 供应商, 签发 Key 成员,
// 新增个人路由规则 就用).
const EMPTY = "__fluxa_empty__"
const fromRadix = (v: string) => (v === EMPTY ? "" : v)

const TRIGGER_BASE =
  "gap-[7px] px-[11px] bg-[var(--panel)] shadow-[0_1px_1px_rgba(22,35,58,.03)] " +
  // The console rings at 14% brand, not shadcn's 50%, and shows the ring
  // while the panel is open -- not only on keyboard focus.
  "focus-visible:ring-[3px] focus-visible:ring-ring/15 " +
  "data-[state=open]:border-ring data-[state=open]:ring-[3px] data-[state=open]:ring-ring/15 " +
  // Upstream tints every un-classed svg muted-grey, which washes out the
  // vendor marks -- they are content, not chrome. Same variant string so
  // twMerge replaces it rather than stacking. Chrome glyphs opt back out
  // by carrying their own `text-` class (see ui/select.tsx).
  "[&_svg:not([class*='text-'])]:text-[var(--ink)]"

const TRIGGER = {
  // Matches .cn-field / .cn-field-select in console.css.
  field: "w-auto data-[size=default]:h-[34px] text-[12.5px] font-[520] text-[var(--ink)] hover:border-[#d3dae6]",
  // Matches .cn-input.
  input: "w-full data-[size=default]:h-9 text-[13px] text-[var(--ink)]",
}

// .cn-menu / .cn-omni: 10px radius, 5px padding, the deeper drop shadow.
// z-[70] clears .cn-modal-scrim's z-60 -- the panel portals to <body>, so
// it shares a stacking context with the dialog and would otherwise open
// behind it.
const CONTENT =
  "z-[70] rounded-lg border-[var(--line)] p-0 " +
  "shadow-[0_1px_2px_rgba(22,35,58,.05),0_18px_40px_-18px_rgba(22,35,58,.34)] " +
  "[&_[data-radix-select-viewport]]:p-[5px]"

// .cn-menu-item, plus a selected row that reads as selected rather than
// only carrying a tick.
const ITEM =
  "gap-2 rounded-[7px] py-[7px] pr-8 pl-[9px] text-[12.5px] text-[var(--ink-2)] " +
  "focus:text-[var(--ink)] data-[state=checked]:text-[var(--ink)] data-[state=checked]:font-[570] " +
  "[&_svg:not([class*='text-'])]:text-[var(--ink)]"

export function Select({
  value,
  onValue,
  options,
  label,
  placeholder = "请选择",
  variant = "field",
  disabled,
  className,
  style,
}: {
  value: string
  onValue: (v: string) => void
  options: SelectOption[]
  /** Accessible name for the control. */
  label?: string
  /** Shown when `value` matches no option. */
  placeholder?: string
  /** `field` is the 34px filter-bar pill, `input` the 36px form control. */
  variant?: "field" | "input"
  disabled?: boolean
  className?: string
  style?: CSSProperties
}) {
  // Only a list that really offers a ""-valued row needs the sentinel;
  // everywhere else "" has to reach Radix intact so the placeholder shows.
  const emptyIsAnOption = options.some((o) => o.value === "")
  const toRadix = (v: string) => (v === "" && emptyIsAnOption ? EMPTY : v)

  return (
    <SelectRoot value={toRadix(value)} onValueChange={(v) => onValue(fromRadix(v))} disabled={disabled}>
      <SelectTrigger
        aria-label={label}
        className={cn(TRIGGER_BASE, TRIGGER[variant], className)}
        style={style}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent
        position="popper"
        align="start"
        className={CONTENT}
      >
        {options.map((o) => (
          <SelectItem key={o.value} value={toRadix(o.value)} disabled={o.disabled} className={ITEM}>
            {o.icon}
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </SelectRoot>
  )
}
