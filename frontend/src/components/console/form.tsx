import type { ComponentProps, CSSProperties } from "react"
import { Checkbox as ShadcnCheckbox } from "@/components/ui/checkbox"
import { Input as ShadcnInput } from "@/components/ui/input"
import { Switch as ShadcnSwitch } from "@/components/ui/switch"
import { Textarea as ShadcnTextarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"

// Console geometry over shadcn's form primitives. Every override here
// closed a gap measured by style-audit against the old `.cn-*` rules --
// shadcn's defaults are a 14px/36px scale with its own focus ring, and
// this console is 12.5-13px with a 14%-opacity brand ring.

const FIELD =
  "h-[36px] rounded-[8px] border-[var(--line)] bg-[var(--panel)] px-[11px] py-0 " +
  "text-[13px] md:text-[13px] text-[var(--ink)] shadow-[0_1px_1px_rgba(22,35,58,.03)] " +
  "placeholder:text-[var(--ink-3)] " +
  "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/15 " +
  // .cn-input:disabled greys the field rather than fading it.
  "disabled:opacity-100 disabled:bg-[#f5f7fb] disabled:text-[var(--ink-3)]"

export function Input({ className, ...props }: ComponentProps<typeof ShadcnInput>) {
  return <ShadcnInput className={cn(FIELD, className)} {...props} />
}

export function Textarea({ className, ...props }: ComponentProps<typeof ShadcnTextarea>) {
  return (
    <ShadcnTextarea
      // textarea.cn-input: auto height, roomier padding, 1.7 line-height.
      // `field-sizing-fixed!` matters -- shadcn ships `field-sizing-content`,
      // which sizes to the text and ignores `rows`, collapsing a rows={2}
      // box from 66px to 44px. twMerge does not know the field-sizing
      // group, so both classes survive the merge and only the important
      // flag settles it.
      className={cn(
        FIELD,
        // `inline-block` is the textarea's own default; shadcn forces
        // `flex`, which makes it block-level.
        "inline-block h-auto min-h-0 resize-y field-sizing-fixed! px-[11px] py-[10px] leading-[1.7]",
        className,
      )}
      {...props}
    />
  )
}

// .cn-switch is 34x20 with a 16px knob travelling 14px. shadcn's default
// is 32x18.4 with a knob sized off its own scale, so both track and thumb
// are restated.
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
    <ShadcnSwitch
      checked={on}
      onCheckedChange={onToggle}
      disabled={disabled || !onToggle}
      aria-label={label}
      style={style}
      className={cn(
        // Sized through the same data-[size] selector shadcn uses, or its
        // 1.15rem/2rem default wins on specificity and the track lands at
        // 18.4px instead of 20.
        "data-[size=default]:h-[20px] data-[size=default]:w-[34px] shrink-0 border-0 shadow-none",
        "data-[state=unchecked]:bg-[#d7dde8] data-[state=checked]:bg-[var(--brand)]",
        "disabled:opacity-50 focus-visible:ring-[3px] focus-visible:ring-ring/20",
        "[&_[data-slot=switch-thumb]]:size-[16px]",
        "[&_[data-slot=switch-thumb]]:bg-white",
        "[&_[data-slot=switch-thumb]]:shadow-[0_1px_2px_rgba(22,35,58,.24)]",
        "[&_[data-slot=switch-thumb][data-state=checked]]:translate-x-[14px]",
      )}
    />
  )
}

// .cn-check is 17px with a 1.5px border and a 5px radius, and has a third
// state the plain checkbox does not: `locked`, a granted-but-immutable
// permission in the role matrix, drawn grey rather than brand.
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
  return (
    <ShadcnCheckbox
      checked={on}
      onCheckedChange={onToggle}
      disabled={!onToggle}
      aria-label={label}
      data-locked={!!(on && locked) || undefined}
      className={cn(
        "inline-flex size-[17px] shrink-0 items-center justify-center rounded-[5px]",
        "border-[1.5px] border-[#ccd4e2] bg-white text-white shadow-none",
        "data-[state=checked]:border-[var(--brand)] data-[state=checked]:bg-[var(--brand)] data-[state=checked]:text-white",
        // `locked` is always also `checked`, and the two rules have equal
        // specificity -- without the important flag Tailwind's ordering
        // decides, and a locked cell paints brand blue instead of grey.
        "data-[locked=true]:border-[#cdd6e6]! data-[locked=true]:bg-[#cdd6e6]!",
        "disabled:cursor-default disabled:opacity-100",
        "focus-visible:ring-[3px] focus-visible:ring-ring/20",
      )}
    />
  )
}
