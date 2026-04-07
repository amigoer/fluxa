// SourceNode — the "request enters here" anchor on the left edge of
// the graph. Read-only, no click handler. Visually it sits in a
// neutral stone palette so the eye flows past it toward the action
// downstream (the regex/virtual/provider colours).

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useT } from "@/lib/i18n";
import type { SourceNodeData } from "../utils/buildGraph";

export function SourceNode(_props: NodeProps & { data: SourceNodeData }) {
  const { t } = useT();
  return (
    <div className="rounded-xl border-[1.5px] bg-[#F1EFE8] dark:bg-zinc-800 border-[#B4B2A9] dark:border-zinc-600 px-3.5 py-2.5 shadow-sm w-[180px]">
      <div className="text-[11px] font-medium text-[#888780] dark:text-zinc-400">
        {t("graph.source.label")}
      </div>
      <div className="mt-0.5 font-mono text-[11px] font-semibold text-[#444441] dark:text-zinc-200">
        model="<span className="text-[#888780] dark:text-zinc-500">*</span>"
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#B4B2A9]"
      />
    </div>
  );
}
