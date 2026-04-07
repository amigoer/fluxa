// ProviderNode — terminal node showing one (provider, model) tuple.
// Health dot is derived from live stats: we look at every inbound edge
// to this node and bucket by the worst error rate among them. When
// live mode is off the dot is gray (unknown).

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useMemo } from "react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { ProviderNodeData } from "../utils/buildGraph";

// Stable colour pick for the provider initial badge. We hash the name
// to one of six pre-picked Tailwind colours so the same provider gets
// the same colour every render without an extra config field.
const BADGE_COLORS = [
  "bg-emerald-500",
  "bg-teal-500",
  "bg-cyan-500",
  "bg-sky-500",
  "bg-blue-500",
  "bg-indigo-500",
];
function badgeColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return BADGE_COLORS[Math.abs(hash) % BADGE_COLORS.length];
}

export function ProviderNode({
  id,
  data,
  selected,
}: NodeProps & { data: ProviderNodeData }) {
  const { t } = useT();
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const liveStats = useRouteGraphStore((s) => s.liveStats);
  const edges = useRouteGraphStore((s) => s.edges);

  // Aggregate health from inbound edges. Worst error rate wins because
  // a single broken upstream should be visible even when others are
  // healthy. This is cheap (linear in edges, ~50 max) so we recompute
  // on every render rather than caching.
  const health = useMemo(() => {
    if (!liveMode) return "unknown" as const;
    const inbound = edges.filter((e) => e.target === id);
    if (inbound.length === 0) return "unknown" as const;
    const worst = inbound.reduce((acc, e) => {
      const s = liveStats[e.id];
      return s && s.errorRate > acc ? s.errorRate : acc;
    }, 0);
    if (worst > 0.05) return "down" as const;
    if (worst > 0.01) return "warn" as const;
    return "ok" as const;
  }, [liveMode, liveStats, edges, id]);

  const dotColor = {
    unknown: "bg-zinc-400",
    ok: "bg-emerald-500",
    warn: "bg-amber-500",
    down: "bg-red-500",
  }[health];

  return (
    <div
      onClick={() => selectNode(id)}
      className={cn(
        "rounded-lg border bg-emerald-50 dark:bg-emerald-950/40 border-emerald-300 dark:border-emerald-800 px-3 py-2.5 shadow-sm w-[220px] cursor-pointer transition-shadow hover:shadow-md",
        selected && "ring-2 ring-purple-500",
      )}
    >
      <div className="flex items-center gap-2.5">
        <div
          className={cn(
            "h-8 w-8 rounded-full text-white text-xs font-semibold uppercase flex items-center justify-center shrink-0",
            badgeColor(data.provider),
          )}
        >
          {data.provider.slice(0, 1)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-xs font-medium text-emerald-900 dark:text-emerald-100 truncate">
            {data.provider}
          </div>
          <div className="text-[10px] font-mono text-emerald-700 dark:text-emerald-300 truncate">
            {data.model}
          </div>
        </div>
        <div
          className={cn("h-2 w-2 rounded-full shrink-0", dotColor)}
          title={t(`graph.errors.status${health.charAt(0).toUpperCase() + health.slice(1)}` as
            | "graph.errors.statusOk"
            | "graph.errors.statusWarn"
            | "graph.errors.statusDown"
            | "graph.errors.statusUnknown")}
        />
      </div>
      <Handle type="target" position={Position.Left} className="!h-2 !w-2 !bg-emerald-500 !border-none" />
    </div>
  );
}
