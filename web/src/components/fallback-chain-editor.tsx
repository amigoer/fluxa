import { useState } from "react";
import { ChevronLeft, ChevronRight, Plus, X, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ProviderIcon } from "@/components/provider-icon";
import { ProviderPicker } from "@/components/provider-picker";
import type { Provider } from "@/lib/api";
import { cn } from "@/lib/utils";

// FallbackChainEditor — an ordered list of provider references shown
// as chips. Each chip can be removed, moved left, or moved right; a
// trailing "+" button opens a ProviderPicker that appends a new entry.
//
// Rendering order IS the runtime order — the first chip is tried
// first, then each successive one if its predecessor fails. Making
// this position-sensitive chain editable via drag-like buttons is a
// massive readability win over the old comma-separated-text input,
// where the operator had to mentally parse "where does the second
// comma go again?" to reorder.
//
// Unknown/deleted provider names render as amber chips so dangling
// references surface visually instead of silently dying at request
// time.

interface Props {
  value: string[];
  onChange: (next: string[]) => void;
  providers: Provider[];
  // Names that should never appear in the picker (typically the
  // route's primary provider, so a fallback chain can't loop back on
  // the node it's falling back from).
  excludeNames?: string[];
}

export function FallbackChainEditor({ value, onChange, providers, excludeNames }: Props) {
  const [adding, setAdding] = useState(false);

  function move(from: number, to: number) {
    if (to < 0 || to >= value.length) return;
    const next = [...value];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    onChange(next);
  }

  function removeAt(idx: number) {
    onChange(value.filter((_, i) => i !== idx));
  }

  function append(name: string) {
    if (!name || value.includes(name)) return;
    onChange([...value, name]);
    setAdding(false);
  }

  // Exclude names already in the chain plus anything the caller asked
  // us to hide (e.g. the primary provider on the route).
  const excluded = [...(excludeNames ?? []), ...value];

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-1.5">
        {value.map((name, idx) => {
          const p = providers.find((x) => x.name === name);
          const dangling = !p;
          return (
            <div
              key={`${name}-${idx}`}
              className={cn(
                "group flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs transition-colors",
                dangling
                  ? "border-amber-500/50 bg-amber-500/5 text-amber-700 dark:text-amber-400"
                  : "border-border bg-accent/40 text-foreground",
              )}
              title={
                dangling
                  ? `Provider "${name}" not found`
                  : `${name} (${p?.kind})`
              }
            >
              {/* Order badge — makes the position in the chain explicit
                  so the operator always knows which one runs first. */}
              <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-background text-[10px] font-semibold text-muted-foreground ring-1 ring-border">
                {idx + 1}
              </span>
              <div className="flex h-4 w-4 shrink-0 items-center justify-center">
                {dangling ? (
                  <AlertTriangle className="h-3.5 w-3.5" />
                ) : (
                  <ProviderIcon kind={p!.kind} className="h-3.5 w-3.5" />
                )}
              </div>
              <span className="max-w-[10rem] truncate font-medium">{name}</span>
              <div className="ml-1 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                <button
                  type="button"
                  onClick={() => move(idx, idx - 1)}
                  disabled={idx === 0}
                  className="rounded p-0.5 hover:bg-background disabled:opacity-30"
                  title="Move earlier"
                >
                  <ChevronLeft className="h-3 w-3" />
                </button>
                <button
                  type="button"
                  onClick={() => move(idx, idx + 1)}
                  disabled={idx === value.length - 1}
                  className="rounded p-0.5 hover:bg-background disabled:opacity-30"
                  title="Move later"
                >
                  <ChevronRight className="h-3 w-3" />
                </button>
                <button
                  type="button"
                  onClick={() => removeAt(idx)}
                  className="rounded p-0.5 text-muted-foreground hover:bg-background hover:text-destructive"
                  title="Remove"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            </div>
          );
        })}

        {!adding && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 gap-1 px-2 text-xs"
            onClick={() => setAdding(true)}
          >
            <Plus className="h-3 w-3" />
            {value.length === 0 ? "Add fallback" : "Add"}
          </Button>
        )}
      </div>

      {adding && (
        <div className="flex items-center gap-2">
          <div className="flex-1">
            <ProviderPicker
              value=""
              onChange={append}
              providers={providers}
              excludeNames={excluded}
              placeholder="Pick a provider…"
              size="sm"
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setAdding(false)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      )}
    </div>
  );
}
