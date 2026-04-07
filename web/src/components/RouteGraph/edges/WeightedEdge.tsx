// WeightedEdge — VirtualModelNode → target. Always renders as a solid
// purple line so the operator instantly distinguishes the "weighted
// fanout" semantic from the dashed-fallback "no match" semantic.
//
// Live mode adds a small set of staggered particles that flow along
// the path via SVG animateMotion, so the operator sees direction of
// flow without breaking the solid-line convention. Particles use the
// `path` attribute on animateMotion (rather than mpath/href) so we do
// not have to worry about element id resolution inside the React Flow
// SVG transform group.

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import type { RouteEdgeData } from "../utils/buildGraph";

const PURPLE = "#7F77DD";

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

  // Label format: weight % from buildGraph (already integer-normalised
  // against the visible-weight total). When live mode is on we append
  // the rolling RPS as a second line so the operator reads the
  // configured share + the actual flow at a glance.
  const pct = data?.weightPct ?? 0;

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{ stroke: PURPLE, strokeWidth: 1.75 }}
      />
      {/* Particle flow — three small dots staggered by a third of the
          animation duration so the eye reads a continuous stream
          rather than a single moving dot. The particles share the
          edge stroke colour so they read as part of the line, not as
          a foreign overlay. */}
      {liveMode &&
        [0, 0.5, 1].map((begin) => (
          <circle
            key={begin}
            r={2.6}
            fill={PURPLE}
            opacity={0.9}
          >
            <animateMotion
              dur="1.5s"
              repeatCount="indefinite"
              path={path}
              begin={`${begin}s`}
            />
          </circle>
        ))}
      <EdgeLabelRenderer>
        <div
          style={{
            position: "absolute",
            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            pointerEvents: "all",
          }}
          className="rounded-md border border-[#AFA9EC] bg-white dark:bg-zinc-900 px-1.5 py-0.5 text-[10px] font-semibold text-[#534AB7] dark:text-purple-300 shadow-sm whitespace-nowrap leading-tight"
        >
          {pct}%
          {liveMode && stat && (
            <span className="ml-1 text-[#7F77DD]/80 font-medium">
              · {Math.round(stat.rps)}rps
            </span>
          )}
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
