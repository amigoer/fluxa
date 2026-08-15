import { cn } from "@/lib/utils"

export function KpiCard({
  label,
  value,
  delta,
  deltaTone,
}: {
  label: string
  value: string
  delta?: string
  deltaTone?: "up" | "down"
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
      <p className="mb-1.5 text-[11px] text-muted-foreground">{label}</p>
      <span className="font-mono text-xl font-semibold tracking-tight text-foreground tabular-nums">{value}</span>
      {delta && (
        <span
          className={cn(
            "ml-1.5 text-[11px] font-semibold",
            deltaTone === "down" ? "text-bad" : "text-ok",
          )}
        >
          {delta}
        </span>
      )}
    </div>
  )
}
