// RegexRouteNode — one of these per configured regex intercept rule.
// Click selects the node and pops the side panel; the inline toggle
// flips the rule's enabled flag without going through the panel for
// the most common operator action.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { RegexNodeData } from "../utils/buildGraph";

export function RegexRouteNode({
  id,
  data,
  selected,
}: NodeProps & { data: RegexNodeData }) {
  const { t } = useT();
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const r = data.route;

  return (
    <div
      onClick={() => selectNode(id)}
      className={cn(
        "rounded-lg border bg-amber-50 dark:bg-amber-950/40 border-amber-300 dark:border-amber-800 px-3 py-2.5 shadow-sm w-[220px] cursor-pointer transition-shadow hover:shadow-md",
        selected && "ring-2 ring-purple-500",
        !r.enabled && "opacity-60",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="font-mono text-[11px] text-amber-900 dark:text-amber-200 break-all leading-tight">
          {r.pattern}
        </div>
        <div
          className={cn(
            "h-2 w-2 shrink-0 rounded-full mt-1",
            r.enabled ? "bg-emerald-500" : "bg-zinc-400",
          )}
          title={r.enabled ? t("graph.regex.statusEnabled") : t("graph.regex.statusDisabled")}
        />
      </div>
      <div className="mt-2 flex items-center justify-between text-[10px]">
        <span className="rounded-sm bg-amber-200 dark:bg-amber-900/60 text-amber-900 dark:text-amber-100 px-1.5 py-0.5 font-mono">
          P{r.priority ?? 100}
        </span>
        <span className="text-amber-700 dark:text-amber-300 font-mono truncate ml-2">
          → {r.target_model}
        </span>
      </div>
      <Handle type="target" position={Position.Left} className="!h-2 !w-2 !bg-amber-500 !border-none" />
      <Handle type="source" position={Position.Right} className="!h-2 !w-2 !bg-amber-500 !border-none" />
    </div>
  );
}
