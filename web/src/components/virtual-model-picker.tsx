import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown, Search, AlertTriangle, Layers } from "lucide-react";
import type { VirtualModel } from "@/lib/api";
import { cn } from "@/lib/utils";

// VirtualModelPicker — the virtual-model sibling of ProviderPicker.
// Shares the exact same layout, keyboard behaviour, and dangling-
// reference treatment so rows that mix real-provider and virtual-
// model targets in the VirtualTargetsEditor look visually consistent.
// The only real differences are the trailing kind chip (replaced with
// a monochrome "virtual" tag and a Layers icon) and the description
// shown as the sub-label.

interface Props {
  value: string;
  onChange: (name: string) => void;
  virtualModels: VirtualModel[];
  placeholder?: string;
  // Names that should never appear in the dropdown. Callers use this
  // to hide the current virtual model from its own targets picker so
  // the most obvious self-loop is structurally impossible.
  excludeNames?: string[];
  disabled?: boolean;
  className?: string;
  size?: "default" | "sm";
}

export function VirtualModelPicker({
  value,
  onChange,
  virtualModels,
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
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    if (open) {
      document.addEventListener("mousedown", handleClickOutside);
      setTimeout(() => inputRef.current?.focus(), 0);
    }
    return () =>
      document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const selected = virtualModels.find((v) => v.name === value);
  const dangling = value !== "" && !selected;

  const excludeSet = new Set(excludeNames ?? []);
  const q = query.trim().toLowerCase();
  const filtered = virtualModels
    .filter((v) => !excludeSet.has(v.name) || v.name === value)
    .filter((v) => {
      if (!q) return true;
      return (
        v.name.toLowerCase().includes(q) ||
        (v.description ?? "").toLowerCase().includes(q)
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
              <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded-[4px] bg-violet-500/10 ring-1 ring-violet-500/30">
                <Layers className="h-3.5 w-3.5 text-violet-600 dark:text-violet-300" />
              </div>
              <span className="truncate font-medium">{selected.name}</span>
              <span className="truncate text-xs text-muted-foreground">
                virtual
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
              {placeholder ?? "Select virtual model"}
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
                {virtualModels.length === 0
                  ? "No virtual models configured"
                  : "No match"}
              </div>
            )}
            {filtered.map((v) => (
              <div
                key={v.name}
                onClick={() => {
                  onChange(v.name);
                  setOpen(false);
                  setQuery("");
                }}
                className={cn(
                  "flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors hover:bg-accent/70",
                  value === v.name
                    ? "bg-accent text-foreground"
                    : "text-foreground/90",
                )}
              >
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-[5px] bg-violet-500/10 shadow-sm ring-1 ring-violet-500/30">
                  <Layers className="h-4 w-4 text-violet-600 dark:text-violet-300" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{v.name}</div>
                  <div className="truncate text-[11px] text-muted-foreground">
                    {v.description?.trim() || "virtual model"}
                    {v.enabled === false && " · disabled"}
                  </div>
                </div>
                {value === v.name && (
                  <Check className="h-4 w-4 shrink-0 text-primary" />
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {dangling && !open && (
        <p className="mt-1 text-[11px] text-amber-600 dark:text-amber-400">
          Virtual model &quot;{value}&quot; not found — route will fail until
          restored.
        </p>
      )}
    </div>
  );
}
