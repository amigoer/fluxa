// NodeSidePanel — slide-in editor that swaps its body based on the
// selected node's type.
//
// Visual model:
//   - The panel floats as a *card* inset 12px from the canvas edges
//     (not edge-to-edge against the viewport) so it reads as an
//     overlay rather than a sibling pane. Rounded corners on all
//     four sides + a soft drop shadow keep it light.
//   - A coloured chip at the top echoes the colour of the node type
//     the operator just clicked, so the panel feels like a literal
//     extension of the node — there is no visual context switch.
//   - Open / close is animated by translating the wrapper. We hold
//     onto the previously displayed node for one transition cycle
//     after the user closes the panel so the body content does not
//     pop out mid-animation.

import { useEffect, useState } from "react";
import { X } from "lucide-react";
import type { Node } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
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

// CHIP_TONE maps each node type to a {dot, label} colour pair so the
// header chip matches the corresponding node card on the canvas. Kept
// in one place so a future palette change only edits one block.
const CHIP_TONE: Record<
  string,
  { dot: string; bg: string; text: string; border: string }
> = {
  regexRoute: {
    dot: "bg-[#EF9F27]",
    bg: "bg-[#FAEEDA]",
    text: "text-[#854F0B]",
    border: "border-[#EF9F27]/40",
  },
  virtualModel: {
    dot: "bg-[#7F77DD]",
    bg: "bg-[#EEEDFE]",
    text: "text-[#3C3489]",
    border: "border-[#AFA9EC]/50",
  },
  provider: {
    dot: "bg-[#5DCAA5]",
    bg: "bg-[#E1F5EE]",
    text: "text-[#085041]",
    border: "border-[#5DCAA5]/40",
  },
  source: {
    dot: "bg-[#B4B2A9]",
    bg: "bg-[#F1EFE8]",
    text: "text-[#5F5E5A]",
    border: "border-[#B4B2A9]/50",
  },
  fallback: {
    dot: "bg-[#B4B2A9]",
    bg: "bg-[#F1EFE8]",
    text: "text-[#5F5E5A]",
    border: "border-[#B4B2A9]/50",
  },
};

export function NodeSidePanel({ onChange }: Props) {
  const { t } = useT();
  const selectedId = useRouteGraphStore((s) => s.selectedNodeId);
  const creatingKind = useRouteGraphStore((s) => s.creatingKind);
  const nodes = useRouteGraphStore((s) => s.nodes);
  const selectNode = useRouteGraphStore((s) => s.selectNode);
  const startCreate = useRouteGraphStore((s) => s.startCreate);

  // displayed mirrors the *currently rendered* intent. It lags the
  // store by one transition cycle on close so the body content does
  // not vanish mid-slide. We track the union of "edit a node" and
  // "create a new X" so the same slide-out behaves the same for
  // both flows.
  const [displayed, setDisplayed] = useState<{
    kind: "edit" | "create";
    node?: Node;
    creatingKind?: "regexRoute" | "virtualModel";
  } | null>(null);

  useEffect(() => {
    if (selectedId) {
      const next = nodes.find((n) => n.id === selectedId);
      if (next) setDisplayed({ kind: "edit", node: next });
      return;
    }
    if (creatingKind) {
      setDisplayed({ kind: "create", creatingKind });
      return;
    }
    // Both intents cleared — keep the current body for the slide-out
    // duration, then drop it so the next open starts cleanly.
    const timer = window.setTimeout(() => setDisplayed(null), 300);
    return () => window.clearTimeout(timer);
  }, [selectedId, creatingKind, nodes]);

  const open = !!selectedId || !!creatingKind;
  // Resolve the type the panel should render. In edit mode it comes
  // from the displayed node; in create mode from the creatingKind tag.
  const resolvedType: string | undefined =
    displayed?.kind === "edit"
      ? displayed.node?.type
      : displayed?.creatingKind;
  const tone =
    (resolvedType && CHIP_TONE[resolvedType]) || CHIP_TONE.source;
  const node = displayed?.kind === "edit" ? displayed.node : undefined;

  // Localized type label and one-line subtitle. The label switches
  // to a "新建 X" form when in create mode so the operator immediately
  // sees the panel is for inserting, not editing.
  const isCreate = displayed?.kind === "create";
  const { typeLabel, subtitle } = (() => {
    if (!resolvedType) return { typeLabel: "", subtitle: "" };
    switch (resolvedType) {
      case "regexRoute":
        return {
          typeLabel: isCreate
            ? t("graph.dialog.newRegex")
            : t("graph.panel.regexTitle"),
          subtitle: t("graph.panel.regexSubtitle"),
        };
      case "virtualModel":
        return {
          typeLabel: isCreate
            ? t("graph.dialog.newVirtual")
            : t("graph.panel.virtualTitle"),
          subtitle: t("graph.panel.virtualSubtitle"),
        };
      case "provider":
        return {
          typeLabel: t("graph.panel.providerTitle"),
          subtitle: t("graph.panel.providerSubtitle"),
        };
      case "source":
        return {
          typeLabel: t("graph.source.label"),
          subtitle: t("graph.synthetic"),
        };
      case "fallback":
        return {
          typeLabel: t("graph.fallback.label"),
          subtitle: t("graph.fallback.hint"),
        };
      default:
        return { typeLabel: resolvedType, subtitle: "" };
    }
  })();

  // close() clears whichever intent is currently open. Edit and
  // create both pass through here so the close button always works
  // regardless of which mode the panel is in.
  const close = () => {
    if (selectedId) selectNode(null);
    if (creatingKind) startCreate(null);
  };

  return (
    // Wrapper is always mounted so the slide-out can animate. When
    // closed we translate it 110% to the right (the extra 10% covers
    // the drop shadow so it never peeks back into the canvas) and
    // drop pointer events so clicks fall through to the canvas.
    <div
      className={cn(
        "absolute top-3 right-3 bottom-3 w-[340px] z-20 transition-all duration-300 ease-out",
        open
          ? "translate-x-0 opacity-100"
          : "translate-x-[calc(100%+1rem)] opacity-0 pointer-events-none",
      )}
    >
      <div className="h-full flex flex-col rounded-2xl border border-border/60 bg-background/95 backdrop-blur-xl shadow-[0_8px_32px_-8px_rgba(0,0,0,0.18)] overflow-hidden">
        {/* Header: coloured chip + subtitle on the left, dismiss
            button anchored on the top-right. The chip carries the
            type label so the operator never has to guess what they
            clicked, and the subtitle line below sets context for the
            form fields immediately under it. */}
        <div className="flex items-start justify-between gap-3 px-4 pt-4 pb-3 border-b border-border/40">
          <div className="min-w-0 space-y-2">
            <div
              className={cn(
                "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-[11px] font-medium",
                tone.bg,
                tone.text,
                tone.border,
              )}
            >
              <span className={cn("h-1.5 w-1.5 rounded-full", tone.dot)} />
              {typeLabel}
            </div>
            {subtitle && (
              <p className="text-[11px] text-muted-foreground leading-snug">
                {subtitle}
              </p>
            )}
          </div>
          <button
            onClick={close}
            className="h-7 w-7 shrink-0 rounded-md inline-flex items-center justify-center text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            aria-label="close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Body — scrollable so long virtual model edits don't push
            the action buttons off-screen. We key the inner panel by
            (mode + node id) so opening a fresh create form resets
            local state cleanly between opens; otherwise React would
            hold onto a previous draft because the same component
            instance is being reused. */}
        <div className="flex-1 overflow-auto px-4 py-4">
          {isCreate && resolvedType === "regexRoute" && (
            <RegexRoutePanel
              key="create-regex"
              onChange={onChange}
              onClose={close}
            />
          )}
          {isCreate && resolvedType === "virtualModel" && (
            <VirtualModelPanel
              key="create-virtual"
              onChange={onChange}
              onClose={close}
            />
          )}
          {!isCreate && node?.type === "regexRoute" && (
            <RegexRoutePanel
              key={`edit-${node.id}`}
              route={(node.data as unknown as RegexNodeData).route}
              onChange={onChange}
              onClose={close}
            />
          )}
          {!isCreate && node?.type === "virtualModel" && (
            <VirtualModelPanel
              key={`edit-${node.id}`}
              model={(node.data as unknown as VirtualModelNodeData).model}
              onChange={onChange}
              onClose={close}
            />
          )}
          {!isCreate && node?.type === "provider" && (
            <ProviderPanel
              data={node.data as unknown as ProviderNodeData}
              nodeId={node.id}
            />
          )}
          {!isCreate &&
            (node?.type === "source" || node?.type === "fallback") && (
              <div className="text-xs text-muted-foreground">
                {t("graph.synthetic")}
              </div>
            )}
        </div>
      </div>
    </div>
  );
}
