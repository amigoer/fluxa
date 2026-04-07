// RouteEdge — solid, non-animated edge used between SourceNode →
// RegexRouteNode and RegexRouteNode → target. Carries a small text
// label (priority for the source-side, "matched" for the target-side)
// so the operator can read the topology without clicking anything.

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import type { RouteEdgeData } from "../utils/buildGraph";

export function RouteEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps & { data?: RouteEdgeData }) {
  const [path, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return (
    <>
      <BaseEdge id={id} path={path} style={{ stroke: "#94a3b8", strokeWidth: 1.5 }} />
      {data?.label && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: "absolute",
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              pointerEvents: "none",
            }}
            className="rounded-sm bg-white/90 dark:bg-zinc-800/90 border border-border/40 px-1.5 py-0 text-[9px] font-mono text-muted-foreground"
          >
            {data.label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}
