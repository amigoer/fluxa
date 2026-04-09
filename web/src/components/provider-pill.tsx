import { AlertTriangle } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";

// ProviderPill — the canonical inline rendering of a provider
// reference used across the routing / virtual-models pages. It
// gracefully handles three states in a single footprint:
//
//   - happy path: icon + name, shows kind on hover
//   - disabled: same chip dimmed with a tiny "off" marker
//   - dangling: amber border + warning triangle (the referenced
//     provider no longer exists, so the router would fail at request
//     time if this row ever runs)
//
// Kept deliberately headless (no layout, no interaction) so it can
// drop into tables, flow diagrams, and inline previews without the
// host having to fight margins.

interface Props {
  name: string;
  kind?: string;
  disabled?: boolean;
  dangling?: boolean;
  small?: boolean;
}

export function ProviderPill({ name, kind, disabled, dangling, small }: Props) {
  const iconSize = small ? "h-3 w-3" : "h-3.5 w-3.5";
  const padding = small ? "px-1.5 py-0.5 text-[11px]" : "px-2 py-1 text-xs";
  if (dangling) {
    return (
      <div
        className={`inline-flex items-center gap-1 rounded-md border border-amber-500/40 bg-amber-500/5 font-medium text-amber-700 dark:text-amber-400 ${padding}`}
        title={`Provider "${name}" not found`}
      >
        <AlertTriangle className={iconSize} />
        <span className="truncate">{name}</span>
      </div>
    );
  }
  return (
    <div
      className={`inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background font-medium ${padding} ${disabled ? "opacity-60" : ""}`}
      title={kind ? `${name} (${kind})` : name}
    >
      {kind && <ProviderIcon kind={kind} className={iconSize} />}
      <span className="truncate">{name}</span>
      {disabled && (
        <span className="rounded bg-muted px-1 text-[9px] text-muted-foreground">
          off
        </span>
      )}
    </div>
  );
}
