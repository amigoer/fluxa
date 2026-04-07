// RouteEdge — non-weighted edges. Three flavours, distinguished by
// data.labelKind:
//
//   - "priority": SourceNode → RegexRouteNode. Solid gray, P{n} label.
//   - "matched":  RegexRouteNode → target. Amber, "matched" label.
//   - "noMatch":  SourceNode → FallbackNode. Gray dashed, "no match".
//
// In live mode every variant flips on the .fluxa-edge-flow class so
// the dash pattern marches in the direction of traffic, matching the
// WeightedEdge animation. The label gains the live RPS suffix when a
// stat sample is available, formatted as "matched · 12rps" so the
// operator reads weight + traffic in one glance.

import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
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
  // for source-side and fallback edges keeps them visually quiet so
  // the action (the orange/purple flow downstream) reads first.
  let stroke = "#94a3b8";
  let baseLabel: string | undefined;
  let dashed = false;
  if (data?.labelKind === "matched") {
    stroke = "#EF9F27";
    baseLabel = t("graph.edge.matched");
  } else if (data?.labelKind === "noMatch") {
    stroke = "#B4B2A9";
    baseLabel = t("graph.edge.noMatch");
    dashed = true;
  } else if (data?.labelKind === "priority") {
    stroke = "#94a3b8";
    baseLabel = `P${data.priority ?? 100}`;
  }

  // Live RPS suffix. We always render the base label (priority / status)
  // and append the rps so the unit reads as one composite badge.
  const label =
    liveMode && stat && baseLabel
      ? `${baseLabel} · ${Math.round(stat.rps)}rps`
      : baseLabel;

  // The static-dashed fallback edge keeps a wider dash pattern when
  // not in live mode so it still reads as "passive". Live mode
  // overrides this with the global .fluxa-edge-flow class which
  // marches dashes from source to target — the dasharray inside the
  // class wins because it carries !important.
  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke,
          strokeWidth: 1.5,
          strokeDasharray: !liveMode && dashed ? "5 4" : undefined,
        }}
        className={cn(liveMode && "fluxa-edge-flow")}
      />
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
