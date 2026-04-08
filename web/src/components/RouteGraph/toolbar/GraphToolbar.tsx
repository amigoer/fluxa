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

  // Shared classes for the plain pill buttons keep every group's
  // visual style identical so the only thing the eye sees is the
  // separator-defined grouping, not stylistic drift between cells.
  const pill =
    "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors";

  return (
    <>
      {/* Centered floating toolbar pinned to the top of the canvas.
          The button list is split into THREE explicit groups by
          vertical separators so the operator can find what they
          want by zone, not by scanning every cell:
            1. Create     — + Regex / + Virtual / + Provider
            2. View       — Auto Layout / Fit View
            3. Live mode  — the toggle (kept on its own so the
                            active "I'm streaming" state is impossible
                            to miss next to the cosmetic view group).
       */}
      <div className="absolute top-3 left-1/2 -translate-x-1/2 z-10 flex items-center gap-1 rounded-xl border border-border/60 bg-background/95 backdrop-blur px-2 py-1.5 shadow-md">
        {/* --- Group 1: create actions ----------------------------- */}
        <button onClick={() => onStartCreate("regexRoute")} className={pill}>
          <RegexIcon className="h-3.5 w-3.5" />
          {t("graph.toolbar.regex")}
        </button>
        <button onClick={() => onStartCreate("virtualModel")} className={pill}>
          <GitBranch className="h-3.5 w-3.5" />
          {t("graph.toolbar.virtual")}
        </button>
        <button onClick={() => onStartCreate("provider")} className={pill}>
          <Server className="h-3.5 w-3.5" />
          {t("graph.toolbar.provider")}
        </button>

        <div className="h-4 w-px bg-border mx-1" />

        {/* --- Group 2: view actions ------------------------------- */}
        <button
          onClick={onLayout}
          title={t("graph.toolbar.autoLayoutTitle")}
          className={pill}
        >
          <RefreshCw className="h-3.5 w-3.5" />
          {t("graph.toolbar.autoLayout")}
        </button>
        <button onClick={() => fitView({ duration: 300 })} className={pill}>
          <Maximize2 className="h-3.5 w-3.5" />
          {t("graph.toolbar.fitView")}
        </button>

        <div className="h-4 w-px bg-border mx-1" />

        {/* --- Group 3: live mode toggle --------------------------- */}
        <button
          onClick={toggleLiveMode}
          className={cn(
            "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors",
            liveMode
              ? "bg-[#FCE9E8] text-[#9B1F1D] border border-[#E24B4A]/40"
              : "text-foreground hover:bg-muted",
          )}
        >
          {/* When live mode is on we stack two dots: a static red
              core + a same-colour ping ring expanding around it.
              The combo reads as "this thing is alive" much louder
              than a plain animate-pulse, which only fades opacity
              and is easy to miss against a coloured background. */}
          <span className="relative inline-flex h-1.5 w-1.5 items-center justify-center">
            {liveMode && (
              <span className="absolute inline-flex h-full w-full rounded-full bg-[#E24B4A] opacity-60 animate-ping" />
            )}
            <span
              className={cn(
                "relative inline-flex h-1.5 w-1.5 rounded-full",
                liveMode ? "bg-[#E24B4A]" : "bg-muted-foreground/40",
              )}
            />
          </span>
          {t("graph.toolbar.live")}
        </button>
      </div>
    </>
  );
}
