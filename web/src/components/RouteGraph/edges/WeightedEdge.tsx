// WeightedEdge — VirtualModelNode → target. Renders the routing weight
// as a percent label in the middle of the edge, and overlays the live
// req/s number when live mode is on. The line itself is drawn by
// React Flow's BaseEdge using a smooth bezier path.

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
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

  // strokeDasharray + the React Flow `animated` style class would
  // collide here, so we just toggle the dasharray ourselves and let
  // React Flow's animation kick in via the explicit class.
  const stroke = liveMode ? "#a855f7" : "#cbd5e1";
  const strokeDasharray = liveMode ? "6 4" : undefined;

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{ stroke, strokeWidth: 2, strokeDasharray }}
        className={liveMode ? "animated" : undefined}
      />
      <EdgeLabelRenderer>
        <div
          style={{
            position: "absolute",
            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            pointerEvents: "all",
          }}
          className="rounded-md bg-white dark:bg-zinc-800 border border-border/60 px-2 py-0.5 text-[10px] font-mono shadow-sm"
        >
          <div className="text-purple-700 dark:text-purple-300 font-semibold">
            {data?.weightPct ?? 0}%
          </div>
          {liveMode && stat && (
            <div className="text-muted-foreground text-[9px] leading-none mt-0.5">
              {stat.rps.toFixed(1)} req/s
            </div>
          )}
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
