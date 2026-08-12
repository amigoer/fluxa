import { TrendingDownIcon, TrendingUpIcon, type LucideIcon } from "lucide-react";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/**
 * A headline figure with its movement against the previous window.
 *
 * `goodDirection` exists because "up" is not universally good here: more
 * requests is healthy, more errors is not, and spend is deliberately
 * neutral — nobody wants the dashboard cheering because costs fell when
 * the real cause was an outage.
 */
export function StatCard({
  label,
  value,
  icon: Icon,
  current,
  previous,
  windowDays,
  goodDirection = "up",
  footer,
  spark,
}: {
  label: string;
  value: string;
  icon: LucideIcon;
  current: number;
  previous: number;
  windowDays: number;
  goodDirection?: "up" | "down" | "neutral";
  footer?: React.ReactNode;
  /** Values normalised to 0..1, oldest first. */
  spark?: number[];
}) {
  const t = useT();
  const delta = percentChange(current, previous);
  const rising = delta !== null && delta > 0;

  const tone =
    delta === null || delta === 0 || goodDirection === "neutral"
      ? "muted"
      : rising === (goodDirection === "up")
        ? "good"
        : "bad";

  return (
    <Card className="gap-0 overflow-hidden py-0">
      <CardHeader className="gap-0 px-5 pt-5 pb-0">
        <div className="flex items-center justify-between gap-2">
          <div className="text-muted-foreground flex items-center gap-2 text-sm font-medium">
            <Icon className="size-4" />
            {label}
          </div>
          {delta !== null ? (
            <span
              className={cn(
                "flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-xs font-medium tabular-nums",
                tone === "good" && "bg-success-subtle text-success-emphasis",
                tone === "bad" && "bg-destructive-subtle text-destructive-emphasis",
                tone === "muted" && "bg-muted text-muted-foreground",
              )}
            >
              {delta !== 0 ? (
                rising ? (
                  <TrendingUpIcon className="size-3" />
                ) : (
                  <TrendingDownIcon className="size-3" />
                )
              ) : null}
              {delta > 0 ? "+" : ""}
              {delta.toFixed(0)}%
            </span>
          ) : null}
        </div>
        <div className="mt-2 text-3xl font-semibold tracking-tight tabular-nums">{value}</div>
      </CardHeader>

      <CardContent className="px-5 pt-1 pb-5">
        <p className="text-muted-foreground text-xs">
          {footer ?? t("overview.vsPrevious", { days: windowDays })}
        </p>
      </CardContent>

      {/* A bar-per-day strip rather than a smoothed sparkline: at seven
          points a line implies a precision the data does not have. */}
      {spark && spark.length > 1 ? (
        <div className="flex h-8 items-end gap-px px-5 pb-4" aria-hidden>
          {spark.map((v, i) => (
            <div
              key={i}
              className="bg-primary/25 flex-1 rounded-t-[2px] last:bg-primary/70"
              style={{ height: `${Math.max(6, v * 100)}%` }}
            />
          ))}
        </div>
      ) : null}
    </Card>
  );
}

/** Null when there is no baseline to compare against. */
function percentChange(current: number, previous: number): number | null {
  if (!previous) return current ? 100 : null;
  return ((current - previous) / previous) * 100;
}

/** Scales a series into 0..1 for the spark strip. */
export function normalise(values: number[]): number[] {
  const max = Math.max(...values, 0);
  if (max <= 0) return values.map(() => 0);
  return values.map((v) => v / max);
}
