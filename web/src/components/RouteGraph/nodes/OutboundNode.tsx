// OutboundNode — terminal "response returns to client" anchor on the
// right edge of the graph. Mirrors SourceNode's role at the opposite
// end, closing the visual loop so the operator sees the full
// request→response lifecycle. Uses sky-blue palette to match the
// "inbound" variant from Routes.tsx EndpointNode.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { LogOut } from "lucide-react";
import { useT } from "@/lib/i18n";

export interface OutboundNodeData {
  label: string;
}

export function OutboundNode(_props: NodeProps & { data: OutboundNodeData }) {
  const { t } = useT();
  return (
    <div className="rounded-xl border-[1.5px] bg-emerald-50/80 dark:bg-emerald-950/30 border-emerald-400/60 dark:border-emerald-700 px-3.5 py-2.5 shadow-sm w-[160px]">
      <div className="flex items-center gap-2">
        <LogOut className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
        <div>
          <div className="text-[10px] font-medium text-emerald-600/80 dark:text-emerald-500 uppercase tracking-wider">
            {t("graph.outbound.subtitle")}
          </div>
          <div className="text-[12px] font-semibold text-emerald-800 dark:text-emerald-200 leading-tight">
            {t("graph.outbound.label")}
          </div>
        </div>
      </div>
      <Handle
        type="target"
        position={Position.Left}
        className="!h-3 !w-3 !bg-white !border-2 !border-emerald-400"
      />
    </div>
  );
}
