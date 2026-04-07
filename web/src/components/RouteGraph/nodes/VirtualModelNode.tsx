// VirtualModelNode — purple "alias with weighted fanout" node. The
// donut is rendered inline as SVG (no chart library) so the node stays
// cheap to render even with dozens of routes. Each route gets its own
// output handle on the right edge so React Flow's edge router can
// space the fanout cleanly instead of bunching every line through one
// point.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { cn } from "@/lib/utils";
import type { VirtualModelNodeData } from "../utils/buildGraph";

// Donut geometry. Radius / stroke chosen so the chart fits comfortably
// inside the 260px-wide node card without dominating it. The dasharray
// trick: each segment is one circle whose stroke-dasharray is set so
// only the segment's arc is visible, then rotated into position via a
// CSS transform on the parent <g>.
const DONUT_RADIUS = 18;
const DONUT_STROKE = 8;
const DONUT_CIRCUMFERENCE = 2 * Math.PI * DONUT_RADIUS;

interface DonutProps {
  weights: number[];
  colors: string[];
}

function Donut({ weights, colors }: DonutProps) {
  const total = weights.reduce((a, b) => a + b, 0) || 1;
  let offset = 0;
  return (
    <svg
      width={56}
      height={56}
      viewBox="0 0 56 56"
      className="shrink-0"
    >
      <g transform="translate(28 28) rotate(-90)">
        {weights.map((w, i) => {
          const fraction = w / total;
          const segLen = fraction * DONUT_CIRCUMFERENCE;
          const dasharray = `${segLen} ${DONUT_CIRCUMFERENCE - segLen}`;
          const dashoffset = -offset;
          offset += segLen;
          return (
            <circle
              key={i}
              r={DONUT_RADIUS}
              cx={0}
              cy={0}
              fill="none"
              stroke={colors[i] ?? "#a855f7"}
              strokeWidth={DONUT_STROKE}
              strokeDasharray={dasharray}
              strokeDashoffset={dashoffset}
            />
          );
        })}
      </g>
    </svg>
  );
}

export function VirtualModelNode({
  id,
  data,
  selected,
}: NodeProps & { data: VirtualModelNodeData }) {
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const vm = data.model;
  const weights = vm.routes.map((r) => r.weight || 0);

  return (
    <div
      onClick={() => selectNode(id)}
      className={cn(
        "rounded-lg border bg-purple-50 dark:bg-purple-950/40 border-purple-300 dark:border-purple-800 px-3 py-2.5 shadow-sm w-[260px] cursor-pointer transition-shadow hover:shadow-md",
        selected && "ring-2 ring-purple-500",
        !vm.enabled && "opacity-60",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="font-medium text-sm text-purple-900 dark:text-purple-100 truncate">
          {vm.name}
        </div>
        <span className="text-[10px] uppercase tracking-wide text-purple-600 dark:text-purple-300 shrink-0">
          virtual
        </span>
      </div>
      <div className="mt-2 flex items-center gap-3">
        <Donut weights={weights} colors={data.colors} />
        <div className="flex-1 min-w-0 space-y-0.5 text-[10px]">
          {vm.routes.slice(0, 4).map((r, i) => (
            <div key={i} className="flex items-center gap-1.5 min-w-0">
              <span
                className="h-2 w-2 rounded-sm shrink-0"
                style={{ background: data.colors[i] }}
              />
              <span className="font-mono truncate text-purple-900 dark:text-purple-100">
                {r.target_model}
              </span>
              <span className="ml-auto text-purple-600 dark:text-purple-300 shrink-0">
                {r.weight}
              </span>
            </div>
          ))}
          {vm.routes.length > 4 && (
            <div className="text-purple-500 italic">
              +{vm.routes.length - 4} more
            </div>
          )}
        </div>
      </div>
      <Handle type="target" position={Position.Left} className="!h-2 !w-2 !bg-purple-500 !border-none" />
      {/* One output handle per route, distributed vertically along the
          right edge so dagre and the edge router can keep the fanout
          tidy. The id matches what buildGraph emitted as
          sourceHandle. */}
      {vm.routes.map((_, idx) => {
        const top = `${((idx + 1) / (vm.routes.length + 1)) * 100}%`;
        return (
          <Handle
            key={idx}
            id={`route-${idx}`}
            type="source"
            position={Position.Right}
            style={{ top }}
            className="!h-2 !w-2 !bg-purple-500 !border-none"
          />
        );
      })}
    </div>
  );
}
