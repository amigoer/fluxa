import type { ReactNode } from "react"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
} from "@/components/ui/dialog"
import { Icon } from "@/components/console/icon"

// The console's dialog, now on Radix underneath. The visible shape is
// unchanged (.cn-modal-scrim / .cn-modal in app.css); what the swap buys
// is the behaviour the hand-rolled version never had: a focus trap,
// focus restored to whatever opened it, `aria-describedby` wiring, and a
// layer stack.
//
// That last one matters most here. The old Modal listened for Escape on
// `window`, which meant a Select opened inside it had to stop the event
// by hand or one Escape would close both. Radix keeps a stack of
// dismissable layers and only the topmost one reacts, so the nesting is
// handled by the primitives rather than by cooperating hacks.

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
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogPortal>
        {/* .cn-modal-scrim: z-60, and a lighter scrim than shadcn's black/50. */}
        <DialogOverlay className="z-[60] bg-[rgba(16,24,40,.34)]" />
        <DialogContent
          showCloseButton={false}
          aria-label={title}
          className={[
            "z-[60] flex max-h-[calc(100%-48px)] flex-col gap-0 overflow-hidden p-0",
            "rounded-[14px] border-[var(--line)] bg-[var(--panel)]",
            "shadow-[0_1px_2px_rgba(22,35,58,.05),0_26px_60px_-26px_rgba(22,35,58,.42)]",
            wide ? "sm:max-w-[560px]" : "sm:max-w-[460px]",
          ].join(" ")}
        >
          <DialogHeader className="flex flex-none flex-row items-center gap-[10px] space-y-0 border-b border-[var(--line-2)] px-[16px] pt-[15px] pb-[13px] text-left">
            <div className="min-w-0">
              <DialogTitle className="text-[14.5px] leading-normal font-[660] tracking-[-0.022em] text-[var(--ink)]">
                {title}
              </DialogTitle>
              {sub ? (
                <DialogDescription className="text-[11.5px] leading-normal text-[var(--ink-3)]">
                  {sub}
                </DialogDescription>
              ) : (
                // Radix warns when a dialog has no description; an empty
                // one keeps the a11y wiring honest without drawing a row.
                <DialogDescription className="sr-only">{title}</DialogDescription>
              )}
            </div>
            <DialogClose
              className="ml-auto inline-flex size-[28px] shrink-0 items-center justify-center rounded-[7px] text-[var(--ink-3)] hover:bg-[#f2f5fa] hover:text-[var(--ink)]"
              aria-label="关闭"
            >
              <Icon name="x" size={16} />
            </DialogClose>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto p-[16px]">{children}</div>

          {footer && (
            <DialogFooter className="flex-none flex-row justify-end gap-[8px] border-t border-[var(--line-2)] bg-[#fbfcfe] px-[16px] py-[12px]">
              {footer}
            </DialogFooter>
          )}
        </DialogContent>
      </DialogPortal>
    </Dialog>
  )
}
