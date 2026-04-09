// RouteEdge — non-weighted edges. Four flavours, distinguished by
// data.labelKind:
//
//   - "priority": SourceNode → RegexModelNode. Solid gray, P{n} label.
//   - "matched":  RegexModelNode → target. Solid amber, "matched".
//   - "direct":   SourceNode → VirtualModelNode (no regex pointed at
//                 it; reachable by direct name match). Solid gray, no
//                 label — the source→VM line tells the whole story.
//   - "noMatch":  SourceNode → FallbackNode. Dashed gray, "no match".
//                 The *only* edge type that uses dashes — dashes are
//                 the dedicated semantic for the catchall path.
//
// In live mode every flavour except noMatch flows particles via SVG
// animateMotion so direction of flow is visible without breaking the
// solid-vs-dashed convention. The fallback edge stays static dashed
// so it remains visually quiet (operators rarely care which exact
// requests fell through, just that the catchall exists).

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
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
  const { t } = useT();
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

  // Per-flavour visual styling. Amber for the regex match line keeps
  // the colour narrative consistent with the regex node body; gray
  // for source-side, direct, and fallback edges keeps them visually
  // quiet so the action (the orange/purple flow downstream) reads
  // first.
  let stroke = "#94a3b8";
  let baseLabel: string | undefined;
  let dashed = false;
  let particles = false;
  if (data?.labelKind === "matched") {
    stroke = "#EF9F27";
    baseLabel = t("graph.edge.matched");
    particles = true;
  } else if (data?.labelKind === "noMatch") {
    stroke = "#B4B2A9";
    baseLabel = t("graph.edge.noMatch");
    dashed = true;
    particles = false;
  } else if (data?.labelKind === "priority") {
    stroke = "#94a3b8";
    baseLabel = `P${data.priority ?? 100}`;
    particles = true;
  } else if (data?.labelKind === "direct") {
    stroke = "#94a3b8";
    baseLabel = undefined;
    particles = true;
  }

  // Live RPS suffix. Only attached when there's a base label to anchor
  // it to — the unlabeled "direct" edges stay clean.
  const label =
    liveMode && stat && baseLabel
      ? `${baseLabel} · ${Math.round(stat.rps)}rps`
      : baseLabel;

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke,
          strokeWidth: 1.5,
          strokeDasharray: dashed ? "6 4" : undefined,
        }}
      />
      {liveMode &&
        particles &&
        [0, 0.5, 1].map((begin) => (
          <circle
            key={begin}
            r={2.2}
            fill={stroke}
            opacity={0.85}
          >
            <animateMotion
              dur="1.5s"
              repeatCount="indefinite"
              path={path}
              begin={`${begin}s`}
            />
          </circle>
        ))}
      {label && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: "absolute",
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              pointerEvents: "none",
            }}
            className="rounded-md border border-border/60 bg-white/95 dark:bg-zinc-900/95 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground shadow-sm whitespace-nowrap"
          >
            {label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}
