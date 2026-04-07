// NodeSidePanel — slide-in container that swaps its body based on the
// selected node's type. Mounted as a sibling of the React Flow canvas
// in the main RouteGraph component; positioning + animation come from
// the wrapper div there.

import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { RegexRoutePanel } from "./RegexRoutePanel";
import { VirtualModelPanel } from "./VirtualModelPanel";
import { ProviderPanel } from "./ProviderPanel";
import type {
  ProviderNodeData,
  RegexNodeData,
  VirtualModelNodeData,
} from "../utils/buildGraph";

interface Props {
  onChange: () => void | Promise<void>;
}

export function NodeSidePanel({ onChange }: Props) {
  const { t } = useT();
  const selectedId = useRouteGraphStore((s) => s.selectedNodeId);
  const nodes = useRouteGraphStore((s) => s.nodes);
  const selectNode = useRouteGraphStore((s) => s.selectNode);

  if (!selectedId) return null;
  const node = nodes.find((n) => n.id === selectedId);
  if (!node) return null;

  const close = () => selectNode(null);

  return (
    <div className="absolute top-0 right-0 h-full w-[320px] bg-background border-l border-border/60 shadow-lg z-10 flex flex-col">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border/60">
        <div className="text-xs uppercase tracking-wide text-muted-foreground">
          {node.type}
        </div>
        <Button variant="ghost" size="icon" onClick={close} className="h-7 w-7">
          <X className="h-4 w-4" />
        </Button>
      </div>
      <div className="flex-1 overflow-auto p-4">
        {node.type === "regexRoute" && (
          <RegexRoutePanel
            route={(node.data as unknown as RegexNodeData).route}
            onChange={onChange}
            onClose={close}
          />
        )}
        {node.type === "virtualModel" && (
          <VirtualModelPanel
            model={(node.data as unknown as VirtualModelNodeData).model}
            onChange={onChange}
            onClose={close}
          />
        )}
        {node.type === "provider" && (
          <ProviderPanel
            data={node.data as unknown as ProviderNodeData}
            nodeId={node.id}
          />
        )}
        {(node.type === "source" || node.type === "fallback") && (
          <div className="text-xs text-muted-foreground">
            {t("graph.synthetic")}
          </div>
        )}
      </div>
    </div>
  );
}
