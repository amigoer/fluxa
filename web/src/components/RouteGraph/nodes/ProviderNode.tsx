// ProviderNode — terminal "concrete upstream" card. Shows a coloured
// initial badge, the provider/model pair, and a status dot whose
// colour is derived from live stats (red on high error rate, green
// when healthy, gray when no data). The whole node is 200px wide so
// the model name has room to breathe.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useMemo } from "react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { ProviderNodeData } from "../utils/buildGraph";

// Stable colour pick for the provider initial badge. We hash the
// provider name to one of six pre-picked tones so the same provider
// gets the same colour every render without an extra config field.
const BADGE_TONES: { bg: string; fg: string }[] = [
  { bg: "#EEEDFE", fg: "#534AB7" }, // purple
  { bg: "#FAECE7", fg: "#993C1D" }, // rust
  { bg: "#E1F5EE", fg: "#085041" }, // emerald
  { bg: "#DFEEFA", fg: "#1E3F73" }, // blue
  { bg: "#FAEEDA", fg: "#854F0B" }, // amber
  { bg: "#FAEAF3", fg: "#922561" }, // pink
];
function badgeTone(name: string) {
  let hash = 0;
  for (let i = 0; i < name.length; i++)
    hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return BADGE_TONES[Math.abs(hash) % BADGE_TONES.length];
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
  // a single broken upstream should be visible even when other paths
  // are healthy. This is cheap (linear in edges, ~50 max) so we
  // recompute on every render rather than caching.
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
    ok: "bg-[#1D9E75]",
    warn: "bg-[#EFB427]",
    down: "bg-[#E24B4A]",
  }[health];

  const tone = badgeTone(data.provider);

  return (
    <div
      onClick={() => selectNode(id)}
      className={cn(
        "rounded-xl border-[1.5px] bg-[#E1F5EE] dark:bg-emerald-950/40 border-[#5DCAA5] dark:border-emerald-700 px-3 py-2.5 shadow-sm w-[200px] cursor-pointer transition-all",
        "hover:shadow-[0_0_0_3px_rgba(127,119,221,0.25)]",
        selected &&
          "shadow-[0_0_0_3px_rgba(127,119,221,0.35)] border-[#7F77DD]",
      )}
    >
      <div className="flex items-center gap-2.5">
        <div
          className="h-7 w-7 rounded-md flex items-center justify-center text-[12px] font-semibold uppercase shrink-0"
          style={{ background: tone.bg, color: tone.fg }}
        >
          {data.provider.slice(0, 1)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-[12px] font-semibold text-[#085041] dark:text-emerald-100 truncate leading-tight">
            {data.provider}
          </div>
          <div className="text-[10px] font-mono text-[#0F6E56] dark:text-emerald-300 truncate mt-0.5">
            {data.model}
          </div>
        </div>
        <div
          className={cn("h-2 w-2 rounded-full shrink-0", dotColor)}
          title={t(
            `graph.errors.status${health.charAt(0).toUpperCase() + health.slice(1)}` as
              | "graph.errors.statusOk"
              | "graph.errors.statusWarn"
              | "graph.errors.statusDown"
              | "graph.errors.statusUnknown",
          )}
        />
      </div>
      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#5DCAA5]"
      />
    </div>
  );
}
