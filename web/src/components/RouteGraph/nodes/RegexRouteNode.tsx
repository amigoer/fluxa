// RegexRouteNode — amber "regex intercept" card. Shows the pattern,
// the priority pill, an enable/disable badge, and a one-line subtitle
// describing where the rule routes to. Click selects the node and
// pops the side panel; the badge is purely informational.
//
// Layout matches the demo: the pattern is the headline (monospace,
// dark amber), the badges sit inline to its right, and the target
// summary lives on a second line in a softer amber tint. The whole
// node is 200px wide so it fits comfortably even with longer
// pattern strings.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { cn } from "@/lib/utils";
import type { RegexNodeData } from "../utils/buildGraph";

export function RegexRouteNode({
  id,
  data,
  selected,
}: NodeProps & { data: RegexNodeData }) {
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const r = data.route;

  return (
    <div
      onClick={() => selectNode(id)}
      className={cn(
        "rounded-xl border-[1.5px] bg-[#FAEEDA] dark:bg-amber-950/40 border-[#EF9F27] dark:border-amber-700 px-3.5 py-2.5 shadow-sm w-[200px] cursor-pointer transition-all",
        "hover:shadow-[0_0_0_3px_rgba(127,119,221,0.25)]",
        selected &&
          "shadow-[0_0_0_3px_rgba(127,119,221,0.35)] border-[#7F77DD]",
        !r.enabled && "opacity-60",
      )}
    >
      {/* Header row: pattern + priority pill + on/off badge.
          The pattern is monospaced and may be long; the flex layout
          lets it shrink with truncate while keeping the pills
          right-anchored. */}
      <div className="flex items-center gap-1.5">
        <span className="font-mono text-[11px] font-semibold text-[#633806] dark:text-amber-200 truncate flex-1">
          {r.pattern}
        </span>
        <span className="bg-[#FAC775] text-[#633806] text-[9px] font-semibold px-1 py-px rounded">
          P{r.priority ?? 100}
        </span>
        <span
          className={cn(
            "text-[9px] font-semibold px-1 py-px rounded",
            r.enabled
              ? "bg-[#9FE1CB] text-[#085041]"
              : "bg-[#D3D1C7] text-[#444441]",
          )}
        >
          {r.enabled ? "on" : "off"}
        </span>
      </div>

      {/* Subtitle: "regex route → target". Reads as a complete
          sentence so the operator instantly knows where the rule
          sends matched traffic without opening the side panel. */}
      <div className="text-[10px] text-[#854F0B] dark:text-amber-300 mt-1 font-mono truncate">
        regex route → {r.target_model}
        {r.target_type === "real" && r.provider ? `@${r.provider}` : ""}
      </div>

      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#EF9F27]"
      />
      <Handle
        type="source"
        position={Position.Right}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#EF9F27]"
      />
    </div>
  );
}
