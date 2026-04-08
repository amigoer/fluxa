// VirtualModelNode — purple "alias with weighted fanout" node.
//
// Originally a donut chart, redesigned to use a horizontal segmented
// weight bar plus inline percentages. The horizontal layout reads
// faster than a donut for the typical 2–4 routes operators configure
// (the eye scans left-to-right naturally and the bar width is
// proportional to traffic share without doing any rotation math).
//
// Each route gets its own output handle on the right edge so React
// Flow's edge router spaces the fanout cleanly instead of bunching
// every line through one point.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { VirtualModelNodeData } from "../utils/buildGraph";

export function VirtualModelNode({
  id,
  data,
  selected,
}: NodeProps & { data: VirtualModelNodeData & { draft?: boolean } }) {
  const { t } = useT();
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const vm = data.model;
  const isDraft = !!data.draft;
  const total =
    vm.routes.reduce((acc, r) => acc + (r.weight || 0), 0) || 1;

  return (
    <div
      // Draft nodes are still being edited in the create panel that
      // is already open — clicking them must not swap the panel
      // into edit mode (which would disable the name field and
      // wipe the form on the next save). Non-draft clicks fall
      // through to the normal edit-selection path.
      onClick={() => {
        if (isDraft) return;
        selectNode(id);
      }}
      className={cn(
        "rounded-xl bg-[#EEEDFE] dark:bg-purple-950/40 px-3.5 py-3 shadow-sm w-[230px] cursor-pointer transition-all",
        isDraft
          ? "border-2 border-dashed border-[#AFA9EC]/80"
          : "border-[1.5px] border-[#AFA9EC] dark:border-purple-700",
        "hover:shadow-[0_0_0_3px_rgba(127,119,221,0.25)]",
        selected &&
          "shadow-[0_0_0_3px_rgba(127,119,221,0.35)] !border-[#7F77DD]",
        !vm.enabled && !isDraft && "opacity-60",
      )}
    >
      {/* Header: name on the left, "virtual" or "draft" pill on the
          right. The name is the user-facing identifier so it gets the
          most weight; the pill is a quiet visual marker of node type
          (or unsaved status, in draft mode). */}
      <div className="flex items-center justify-between gap-2">
        <div
          className={cn(
            "text-[13px] font-semibold truncate",
            vm.name
              ? "text-[#3C3489] dark:text-purple-100"
              : "text-[#3C3489]/50 italic",
          )}
        >
          {vm.name || t("graph.draft.virtualPlaceholder")}
        </div>
        <span
          className={cn(
            "text-[9px] uppercase tracking-wide shrink-0 font-medium",
            isDraft
              ? "rounded bg-[#7F77DD] text-white px-1 py-px normal-case"
              : "text-[#7F77DD] dark:text-purple-300",
          )}
        >
          {isDraft ? t("graph.draft.badge") : t("graph.virtual.badge")}
        </span>
      </div>

      {/* Horizontal weight bar — one segment per route, width
          proportional to its share of the total weight. Segments use
          the per-route palette computed in buildGraph so the colours
          stay stable across edge labels and the side panel. */}
      <div className="mt-2.5 flex h-2 overflow-hidden rounded-full bg-white/60 dark:bg-purple-900/40">
        {vm.routes.map((r, i) => {
          const pct = ((r.weight || 0) / total) * 100;
          return (
            <div
              key={i}
              style={{
                width: `${pct}%`,
                background: data.colors[i],
              }}
              className="h-full"
            />
          );
        })}
      </div>

      {/* Inline percentage labels coloured to match each segment.
          We render at most four labels in the body and overflow into
          a "+N" tail so the node never grows wider than 230px. */}
      <div className="mt-1.5 flex items-center gap-1.5 text-[10px] font-semibold">
        {vm.routes.slice(0, 4).map((r, i) => {
          const pct = Math.round(((r.weight || 0) / total) * 100);
          return (
            <span key={i} style={{ color: data.colors[i] }}>
              {pct}%
            </span>
          );
        })}
        {vm.routes.length > 4 && (
          <span className="text-[#7F77DD]/70 italic">
            {t("graph.virtual.more", { count: vm.routes.length - 4 })}
          </span>
        )}
      </div>

      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#AFA9EC]"
      />
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
            className="!h-3 !w-3 !bg-white !border-2 !border-[#AFA9EC]"
          />
        );
      })}
    </div>
  );
}
