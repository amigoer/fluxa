// SourceNode — the "request enters here" anchor on the left edge of
// the graph. Not editable; clicking it does nothing useful, so we do
// not register a click handler.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { LogIn } from "lucide-react";
import { useT } from "@/lib/i18n";
import type { SourceNodeData } from "../utils/buildGraph";

export function SourceNode(_props: NodeProps & { data: SourceNodeData }) {
  const { t } = useT();
  return (
    <div className="rounded-lg border border-border/60 border-l-4 border-l-zinc-500 bg-white dark:bg-zinc-800 px-4 py-3 shadow-sm w-[220px]">
      <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase tracking-wide">
        <LogIn className="h-3.5 w-3.5" />
        {t("graph.source.label")}
      </div>
      <div className="mt-1 font-mono text-xs text-foreground">
        model="<span className="text-zinc-500">{"{name}"}</span>"
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!h-2 !w-2 !bg-zinc-400 !border-none"
      />
    </div>
  );
}
