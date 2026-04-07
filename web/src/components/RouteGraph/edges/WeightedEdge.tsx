// WeightedEdge — VirtualModelNode → target. The label combines the
// configured weight (always shown) with the live RPS (only when live
// mode is on) so the operator reads "30% · 43rps" as one tight unit
// instead of two stacked numbers. The line itself is drawn by React
// Flow's BaseEdge using a smooth bezier path.
//
// Animation: when live mode is on we add the .fluxa-edge-flow class
// (defined in the global <style> block injected by RouteGraph/index)
// so the dash pattern marches from source to target. We do NOT use
// React Flow's built-in `animated` flag because we want full control
// over dash sizing and timing — the default 5px dashes at 0.5s look
// jittery on long edges.

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { cn } from "@/lib/utils";
import type { RouteEdgeData } from "../utils/buildGraph";

export function WeightedEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps & { data?: RouteEdgeData }) {
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const stat = useRouteGraphStore((s) => s.liveStats[id]);

  const [path, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  // Tight inline format: "30%" when static, "30% · 43rps" when live.
  // Using `·` rather than a dash keeps the badge narrow on screens
  // where the operator has many fanout edges in view at once.
  const pct = data?.weightPct ?? 0;
  const label =
    liveMode && stat
      ? `${pct}% · ${Math.round(stat.rps)}rps`
      : `${pct}%`;

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke: "#7F77DD",
          strokeWidth: 1.75,
        }}
        className={cn(liveMode && "fluxa-edge-flow")}
      />
      <EdgeLabelRenderer>
        <div
          style={{
            position: "absolute",
            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            pointerEvents: "all",
          }}
          className="rounded-md border border-purple-200 dark:border-purple-800 bg-white dark:bg-zinc-900 px-1.5 py-0.5 text-[10px] font-semibold text-purple-700 dark:text-purple-300 shadow-sm whitespace-nowrap"
        >
          {label}
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
