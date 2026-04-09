import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown, Search, AlertTriangle } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import type { Provider } from "@/lib/api";
import { cn } from "@/lib/utils";

// ProviderPicker — a searchable dropdown for selecting a named Provider
// *instance* (e.g. "openai-prod", "azure-eu") from the configured list.
// Distinct from the `kind` selector in Providers.tsx, which picks a
// vendor type (openai / azure / …). Routes, fallback chains, virtual
// models, and regex routes all need to reference a concrete provider by
// name — this component standardises that picker so icons, disabled
// states, and dangling-reference warnings look the same everywhere.
//
// The `value` prop is the provider name (string). If the name doesn't
// match any entry in `providers` the trigger still renders the text so
// the operator can see the dangling reference, and a warning icon is
// shown so it's obvious something is off.
//
// `excludeNames` lets callers hide providers already picked elsewhere
// (e.g. a fallback chain hiding providers already on the chain).

interface Props {
  value: string;
  onChange: (name: string) => void;
  providers: Provider[];
  placeholder?: string;
  excludeNames?: string[];
  disabled?: boolean;
  className?: string;
  // When set, disables the trigger and renders it as an inline-sized
  // button suitable for dropping inside a table row or chip picker.
  size?: "default" | "sm";
}

export function ProviderPicker({
  value,
  onChange,
  providers,
  placeholder,
  excludeNames,
  disabled,
  className,
  size = "default",
}: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) {
      document.addEventListener("mousedown", handleClickOutside);
      // Focus the search box as soon as the menu opens so the user
      // can start typing immediately.
      setTimeout(() => inputRef.current?.focus(), 0);
    }
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const selected = providers.find((p) => p.name === value);
  // A "dangling" reference is a provider name that no longer resolves
  // to an entry in the list — typically because the provider was
  // renamed or deleted. We still render the text so the operator can
  // see what was persisted, but flag it visually.
  const dangling = value !== "" && !selected;

  const excludeSet = new Set(excludeNames ?? []);
  const q = query.trim().toLowerCase();
  const filtered = providers
    .filter((p) => !excludeSet.has(p.name) || p.name === value)
    .filter((p) => {
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        p.kind.toLowerCase().includes(q)
      );
    });

  return (
    <div className={cn("relative", className)} ref={containerRef}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          "flex w-full items-center justify-between rounded-md border border-input bg-background px-3 text-sm transition-colors hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
          size === "sm" ? "h-8" : "h-[38px] py-2",
        )}
      >
        <div className="flex min-w-0 items-center gap-2 text-foreground/90">
          {selected ? (
            <>
              <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded-[4px] bg-accent/80 ring-1 ring-border/50">
                <ProviderIcon kind={selected.kind} className="h-3.5 w-3.5" />
              </div>
              <span className="truncate font-medium">{selected.name}</span>
              <span className="truncate text-xs text-muted-foreground">
                {selected.kind}
              </span>
              {selected.enabled === false && (
                <span className="rounded bg-muted px-1 text-[10px] text-muted-foreground">
                  disabled
                </span>
              )}
            </>
          ) : value ? (
            <>
              <AlertTriangle className="h-4 w-4 shrink-0 text-amber-500" />
              <span className="truncate text-amber-600 dark:text-amber-400">
                {value}
              </span>
            </>
          ) : (
            <span className="text-muted-foreground">
              {placeholder ?? "Select provider"}
            </span>
          )}
        </div>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 opacity-50 transition-transform",
            open && "rotate-180",
          )}
        />
      </button>

      {open && (
        <div className="absolute top-[calc(100%+6px)] z-[100] w-full overflow-hidden rounded-lg border border-border bg-background shadow-xl animate-in fade-in-0 zoom-in-95 backdrop-blur-xl">
          <div className="flex items-center gap-2 border-b border-border/60 px-3 py-2">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search…"
              className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
          <div className="max-h-[18rem] overflow-y-auto p-1.5">
            {filtered.length === 0 && (
              <div className="px-2.5 py-4 text-center text-xs text-muted-foreground">
                {providers.length === 0 ? "No providers configured" : "No match"}
              </div>
            )}
            {filtered.map((p) => (
              <div
                key={p.name}
                onClick={() => {
                  onChange(p.name);
                  setOpen(false);
                  setQuery("");
                }}
                className={cn(
                  "flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors hover:bg-accent/70",
                  value === p.name
                    ? "bg-accent text-foreground"
                    : "text-foreground/90",
                )}
              >
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-[5px] bg-card shadow-sm ring-1 ring-border/40">
                  <ProviderIcon kind={p.kind} className="h-4 w-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{p.name}</div>
                  <div className="truncate text-[11px] text-muted-foreground">
                    {p.kind}
                    {p.enabled === false && " · disabled"}
                  </div>
                </div>
                {value === p.name && (
                  <Check className="h-4 w-4 shrink-0 text-primary" />
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {dangling && !open && (
        <p className="mt-1 text-[11px] text-amber-600 dark:text-amber-400">
          Provider &quot;{value}&quot; not found — route will fail until restored.
        </p>
      )}
    </div>
  );
}
