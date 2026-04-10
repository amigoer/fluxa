// FallbackNode — terminal "no rule matched" branch. Always present so
// the operator can visually confirm there is a default path even when
// the regex model table is empty. Renders with a dashed border to
// signal "passive / catch-all" without competing with the active
// coloured nodes upstream.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useT } from "@/lib/i18n";
import type { FallbackNodeData } from "../utils/buildGraph";

export function FallbackNode(_props: NodeProps & { data: FallbackNodeData }) {
  const { t } = useT();
  return (
    <div className="rounded-xl border-[1.5px] border-dashed bg-[#F1EFE8]/60 dark:bg-zinc-900/40 border-[#B4B2A9] dark:border-zinc-600 px-3.5 py-2.5 w-[200px]">
      <div className="text-[11px] font-semibold text-[#5F5E5A] dark:text-zinc-200">
        {t("graph.fallback.label")}
      </div>
      <div className="text-[10px] text-[#888780] dark:text-zinc-500 mt-0.5">
        {t("graph.fallback.hint")}
      </div>
      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#B4B2A9]"
      />
      <Handle
        type="source"
        position={Position.Right}
        className="!h-3 !w-3 !bg-white !border-2 !border-[#B4B2A9]"
      />
    </div>
  );
}
