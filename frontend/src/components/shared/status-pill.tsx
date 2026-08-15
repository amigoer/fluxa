import { cn } from "@/lib/utils"

export type PillTone = "ok" | "bad" | "warn"

export function StatusPill({ tone, children }: { tone: PillTone; children: React.ReactNode }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2 py-[3px] text-[10.5px] font-semibold",
        tone === "ok" && "bg-ok-soft text-ok",
        tone === "bad" && "bg-bad-soft text-bad",
        tone === "warn" && "bg-warn-soft text-warn",
      )}
    >
      <span
        className={cn(
          "size-[5px] rounded-full",
          tone === "ok" && "bg-ok",
          tone === "bad" && "bg-bad",
          tone === "warn" && "bg-warn",
        )}
      />
      {children}
    </span>
  )
}
