// FallbackNode — terminal "no rule matched" branch. Always present, so
// the operator can visually confirm there is a default path even when
// the regex route table is empty.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { FallbackNodeData } from "../utils/buildGraph";

export function FallbackNode({ data }: NodeProps & { data: FallbackNodeData }) {
  return (
    <div className="rounded-lg border-2 border-dashed border-zinc-300 dark:border-zinc-700 bg-white/50 dark:bg-zinc-900/40 px-3 py-2.5 w-[220px]">
      <div className="text-xs font-medium text-zinc-700 dark:text-zinc-200">
        {data.label}
      </div>
      <div className="text-[10px] text-muted-foreground mt-0.5">
        original model name forwarded
      </div>
      <Handle type="target" position={Position.Left} className="!h-2 !w-2 !bg-zinc-400 !border-none" />
    </div>
  );
}
