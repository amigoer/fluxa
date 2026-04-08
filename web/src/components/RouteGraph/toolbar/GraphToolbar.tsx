// GraphToolbar — floating action bar pinned to the top of the canvas.
// Hosts the create-new-rule entry points, the manual layout reset, the
// live mode toggle (with a pulsing dot when active), and the React
// Flow fitView shortcut. The create flows are now hosted inside the
// shared NodeSidePanel — we just dispatch a `startCreate` and the
// panel slides in with the matching empty form.

import {
  Maximize2,
  RefreshCw,
  GitBranch,
  Regex as RegexIcon,
  Server,
} from "lucide-react";
import { useReactFlow } from "@xyflow/react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface Props {
  onLayout: () => void;
  // onChange is unused now (the side panel calls load() directly via
  // its own onChange prop) but we keep the prop in the interface so
  // the parent's wiring stays the same — easier to add new toolbar
  // actions later that *do* need a refresh hook.
  onChange?: () => void | Promise<void>;
  // onStartCreate opens the side panel in create mode for the
  // given kind. For regex/virtual it also inserts a draft node
  // on the canvas; for provider there is no draft node (providers
  // are config blobs, not (provider, model) tuples). The parent
  // owns the node insertion because it has the position context.
  onStartCreate: (kind: "regexRoute" | "virtualModel" | "provider") => void;
}

export function GraphToolbar({ onLayout, onStartCreate }: Props) {
  const { t } = useT();
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const toggleLiveMode = useRouteGraphStore((s) => s.toggleLiveMode);
  const { fitView } = useReactFlow();

  return (
    <>
      {/* Centered floating toolbar pinned to the top of the canvas.
          Each button is rendered as a flat pill so the toolbar reads
          as one unit; vertical separators carve it into logical
          groups (create / layout / live / view). The Live button
          gets a special active treatment — purple background plus a
          pulsing red dot — so the operator instantly sees that
          edges are animated because *they turned it on*. */}
      <div className="absolute top-3 left-1/2 -translate-x-1/2 z-10 flex items-center gap-1 rounded-xl border border-border/60 bg-background/95 backdrop-blur px-2 py-1.5 shadow-md">
        <button
          onClick={() => onStartCreate("regexRoute")}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <RegexIcon className="h-3.5 w-3.5" />
          {t("graph.toolbar.regex")}
        </button>
        <button
          onClick={() => onStartCreate("virtualModel")}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <GitBranch className="h-3.5 w-3.5" />
          {t("graph.toolbar.virtual")}
        </button>
        <button
          onClick={() => onStartCreate("provider")}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <Server className="h-3.5 w-3.5" />
          {t("graph.toolbar.provider")}
        </button>
        <div className="h-4 w-px bg-border mx-0.5" />
        <button
          onClick={onLayout}
          title={t("graph.toolbar.autoLayoutTitle")}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          {t("graph.toolbar.autoLayout")}
        </button>
        <div className="h-4 w-px bg-border mx-0.5" />
        <button
          onClick={toggleLiveMode}
          className={cn(
            "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors",
            liveMode
              ? "bg-[#EEEDFE] text-[#3C3489] border border-[#AFA9EC]/60"
              : "text-foreground hover:bg-muted",
          )}
        >
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              liveMode ? "bg-[#E24B4A] animate-pulse" : "bg-muted-foreground/40",
            )}
          />
          {t("graph.toolbar.live")}
        </button>
        <div className="h-4 w-px bg-border mx-0.5" />
        <button
          onClick={() => fitView({ duration: 300 })}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <Maximize2 className="h-3.5 w-3.5" />
          {t("graph.toolbar.fitView")}
        </button>
      </div>
    </>
  );
}
