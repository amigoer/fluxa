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

interface ActionBtnProps {
  onClick: () => void;
  label: string;
  className?: string;
  children: React.ReactNode;
}

// Shared component for the toolbar buttons. Keeps every group's
// visual style identical and automatically handles the mobile
// collapse logic (hiding text and enabling a custom CSS tooltip).
function ActionBtn({ onClick, label, children, className }: ActionBtnProps) {
  return (
    <button
      onClick={onClick}
      className={cn("inline-flex shrink-0 whitespace-nowrap items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors group relative", className)}
      title={label}
    >
      {children}
      <span className="hidden md:inline">{label}</span>
      {/* CSS Tooltip: visible only on mobile/compact (md:hidden) when hovered */}
      <div className={cn(
        "absolute top-full mt-1.5 left-1/2 -translate-x-1/2 hidden group-hover:flex md:group-hover:hidden px-2.5 py-1 bg-zinc-900 dark:bg-zinc-800 border border-zinc-700 text-white text-[11px] font-normal tracking-wide rounded-md shadow-xl whitespace-nowrap z-50 pointer-events-none"
      )}>
        {label}
      </div>
    </button>
  );
}

export function GraphToolbar({ onLayout, onStartCreate }: Props) {
  const { t } = useT();
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const toggleLiveMode = useRouteGraphStore((s) => s.toggleLiveMode);
  const { fitView } = useReactFlow();

  const plainPill = "text-foreground hover:bg-muted";

  return (
    <>
      {/* Centered floating toolbar pinned to the top of the canvas.
          The button list is split into THREE explicit groups by
          vertical separators so the operator can find what they
          want by zone, not by scanning every cell:
            1. Create     — + Regex / + Virtual / + Provider
            2. View       — Auto Layout / Fit View
                            active "I'm streaming" state is impossible
                            to miss next to the cosmetic view group).
       */}
      <div className="absolute top-3 left-1/2 -translate-x-1/2 z-10 flex items-center gap-1 rounded-xl border border-border/60 bg-background/95 backdrop-blur px-2 py-1.5 shadow-md">
        <ActionBtn
          onClick={() => onStartCreate("regexRoute")}
          label={t("graph.toolbar.regex")}
          className={plainPill}
        >
          <RegexIcon className="h-3.5 w-3.5" />
        </ActionBtn>
        <ActionBtn
          onClick={() => onStartCreate("virtualModel")}
          label={t("graph.toolbar.virtual")}
          className={plainPill}
        >
          <GitBranch className="h-3.5 w-3.5" />
        </ActionBtn>
        <ActionBtn
          onClick={() => onStartCreate("provider")}
          label={t("graph.toolbar.provider")}
          className={plainPill}
        >
          <Server className="h-3.5 w-3.5" />
        </ActionBtn>

        <div className="h-4 w-px bg-border mx-1" />

        {/* --- Group 2: view actions ------------------------------- */}
        <ActionBtn
          onClick={onLayout}
          label={t("graph.toolbar.autoLayout")}
          className={plainPill}
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </ActionBtn>
        <ActionBtn
          onClick={() => fitView({ duration: 300 })}
          label={t("graph.toolbar.fitView")}
          className={plainPill}
        >
          <Maximize2 className="h-3.5 w-3.5" />
        </ActionBtn>

        <div className="h-4 w-px bg-border mx-1" />

        {/* --- Group 3: live mode toggle --------------------------- */}
        <ActionBtn
          onClick={toggleLiveMode}
          label={t("graph.toolbar.live")}
          className={liveMode
            ? "bg-[#FCE9E8] text-[#9B1F1D] border border-[#E24B4A]/40"
            : plainPill}
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
        </ActionBtn>
      </div>
    </>
  );
}
