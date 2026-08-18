import type { ComponentProps } from "react"
import { Button as ShadcnButton } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// The console's button geometry on top of shadcn's Button. shadcn is
// built on a 14px/36px scale; this console runs at 12.5px/34px, so every
// wrapper here restates size, weight and palette from console.css.
//
// The `tone` names are the console's, not shadcn's, so call sites read
// the same as the classes they replace:
//   default  .cn-btn            bordered white, the workhorse
//   primary  .cn-btn.cn-btn-pri brand fill
//   mini     .cn-mini-btn       27px, used inside dense cards
//   miniPri  .cn-mini-btn.cn-mini-pri
//   icon     .cn-icon-act       26px square, row-level actions
//   link     .cn-link-act       text-only action

// `whitespace-normal` and the py-0 / has-[>svg] overrides look pedantic
// but each one closed a measured gap against the old CSS: shadcn adds
// `whitespace-nowrap`, `py-2`, and a narrower `has-[>svg]:px-3` that the
// console never had. See style-audit for the before/after.
const BASE =
  "shrink-0 whitespace-normal py-0 transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/15 " +
  // shadcn forces un-classed glyphs to 16px. The console's Icon carries
  // its own width/height per call site (13/14/16px), and letting size-4
  // win silently widened every icon button's label by 2px.
  "[&_svg:not([class*='size-'])]:size-auto"

const TONE = {
  default:
    "h-[34px] gap-[6px] rounded-[8px] px-[13px] has-[>svg]:px-[13px] text-[12.5px] font-[560] " +
    "border-[var(--line)] bg-white text-[var(--ink)] shadow-[0_1px_1px_rgba(22,35,58,.04)] " +
    "hover:border-[#d3dae6] hover:bg-white hover:text-[var(--ink)] " +
    "disabled:opacity-55 disabled:hover:border-[var(--line)]",
  primary:
    "h-[34px] gap-[6px] rounded-[8px] px-[13px] has-[>svg]:px-[13px] text-[12.5px] font-[560] " +
    "border border-[var(--brand)] bg-[var(--brand)] text-white shadow-[0_1px_1px_rgba(22,35,58,.04)] " +
    "hover:bg-[#4f46e5] hover:border-[#4f46e5] " +
    "disabled:opacity-55 disabled:hover:bg-[var(--brand)] disabled:hover:border-[var(--brand)]",
  mini:
    "h-[27px] gap-0 rounded-[7px] px-[10px] has-[>svg]:px-[10px] text-[11.5px] font-[570] shadow-none " +
    "border-[var(--line)] bg-white text-[var(--ink)] " +
    "hover:border-[#d3dae6] hover:bg-[#fbfcfe] hover:text-[var(--ink)] disabled:opacity-55",
  miniPri:
    "h-[27px] gap-0 rounded-[7px] px-[10px] has-[>svg]:px-[10px] text-[11.5px] font-[570] shadow-none " +
    "border border-[var(--ink)] bg-[var(--ink)] text-white " +
    "hover:bg-[#23324c] hover:border-[#23324c] disabled:opacity-55",
  icon:
    // No font rules on .cn-icon-act, so it inherits the body's -- shadcn's
    // text-sm/font-medium would not have.
    "size-[26px] gap-0 rounded-[6px] text-[13px] font-normal leading-[1.5] text-[var(--ink-3)] " +
    "hover:bg-[#f2f5fa] hover:text-[var(--ink)] " +
    // .cn-icon-act[data-danger="true"]
    "data-[danger=true]:hover:bg-[var(--bad-soft)] data-[danger=true]:hover:text-[var(--bad)]",
  link: "h-auto gap-0 rounded-none p-0 text-[12px] font-[560] leading-[1.5] text-[var(--brand)] hover:underline",
} as const

// shadcn's own variant underneath each tone: it decides the parts the
// console CSS does not restate (border presence, hover model).
const VARIANT = {
  default: "outline",
  primary: "default",
  mini: "outline",
  miniPri: "default",
  icon: "ghost",
  link: "link",
} as const

export type ButtonTone = keyof typeof TONE

export function Button({
  tone = "default",
  className,
  ...props
}: Omit<ComponentProps<typeof ShadcnButton>, "variant" | "size"> & { tone?: ButtonTone }) {
  return (
    <ShadcnButton
      variant={VARIANT[tone]}
      size={tone === "icon" ? "icon" : "default"}
      className={cn(BASE, TONE[tone], className)}
      {...props}
    />
  )
}
