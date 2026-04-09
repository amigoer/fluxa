import { Plus, X, Power, Box, Layers } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ProviderPicker } from "@/components/provider-picker";
import { VirtualModelPicker } from "@/components/virtual-model-picker";
import type { Provider, VirtualModel, VirtualModelRoute } from "@/lib/api";
import { cn } from "@/lib/utils";

// VirtualTargetsEditor — the power-user variant of
// WeightedTargetsEditor. Two extra capabilities on top of the Routes
// page split editor:
//
//   1. Target type switch: each row can point at a real provider or
//      at another virtual model (recursive, capped at 5 hops in the
//      backend resolver). The Routes page deliberately hides this to
//      keep the common case one-line, but virtual model composition
//      is the whole reason this page exists.
//
//   2. Per-row enabled toggle: operators can park a target without
//      deleting the row. Useful for temporary incidents — "take
//      ollama out of rotation for an hour" — where losing the weight
//      + provider pair would mean retyping it later.
//
// Virtual targets are rendered with a Layers icon to visually
// separate them from real-provider rows, so at a glance the operator
// can see which rows recurse and which land directly on upstream.

interface Props {
  value: VirtualModelRoute[];
  onChange: (next: VirtualModelRoute[]) => void;
  providers: Provider[];
  virtualModels: VirtualModel[];
  // Name of the current virtual model being edited, so we can exclude
  // it from its own virtual-target dropdown (no self-loops).
  selfName?: string;
}

export function VirtualTargetsEditor({
  value,
  onChange,
  providers,
  virtualModels,
  selfName,
}: Props) {
  const total = value.reduce(
    (acc, r) => acc + (r.enabled === false ? 0 : Math.max(0, r.weight || 0)),
    0,
  );

  function update(idx: number, patch: Partial<VirtualModelRoute>) {
    onChange(value.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  }

  function removeAt(idx: number) {
    onChange(value.filter((_, i) => i !== idx));
  }

  function append() {
    onChange([
      ...value,
      {
        weight: 1,
        target_type: "real",
        target_model: "",
        provider: "",
        enabled: true,
        position: value.length,
      },
    ]);
  }

  // Virtual targets may not point at the current virtual model —
  // trivial self-loop. Deeper cycles (A→B→A) are still caught
  // server-side by the resolver's hop cap.
  const virtualExclude = selfName ? [selfName] : [];

  return (
    <div className="space-y-2">
      {value.length === 0 && (
        <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-center text-xs text-muted-foreground">
          No targets yet. Add at least one to route traffic.
        </div>
      )}

      {value.map((row, idx) => {
        const isReal = row.target_type === "real";
        const isOff = row.enabled === false;
        const effectiveWeight = isOff ? 0 : Math.max(0, row.weight || 0);
        const share = total > 0 ? (effectiveWeight / total) * 100 : 0;

        return (
          <div
            key={idx}
            className={cn(
              "space-y-2 rounded-md border bg-muted/20 p-2.5 transition-opacity",
              isOff
                ? "border-border/40 opacity-60"
                : "border-border/60",
            )}
          >
            {/* Row 1: target_type toggle + enabled + remove */}
            <div className="flex items-center gap-2">
              <div className="flex overflow-hidden rounded-md border border-border/60">
                <button
                  type="button"
                  onClick={() =>
                    update(idx, {
                      target_type: "real",
                      target_model: "",
                      provider: "",
                    })
                  }
                  className={cn(
                    "flex items-center gap-1 px-2 py-1 text-[11px] font-medium transition-colors",
                    isReal
                      ? "bg-primary text-primary-foreground"
                      : "bg-background text-muted-foreground hover:bg-accent",
                  )}
                >
                  <Box className="h-3 w-3" /> real
                </button>
                <button
                  type="button"
                  onClick={() =>
                    update(idx, {
                      target_type: "virtual",
                      target_model: "",
                      provider: "",
                    })
                  }
                  className={cn(
                    "flex items-center gap-1 border-l border-border/60 px-2 py-1 text-[11px] font-medium transition-colors",
                    !isReal
                      ? "bg-primary text-primary-foreground"
                      : "bg-background text-muted-foreground hover:bg-accent",
                  )}
                >
                  <Layers className="h-3 w-3" /> virtual
                </button>
              </div>

              <div className="flex-1" />

              <button
                type="button"
                onClick={() => update(idx, { enabled: !(row.enabled ?? true) })}
                className={cn(
                  "flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
                  isOff
                    ? "border-border/40 bg-background text-muted-foreground"
                    : "border-emerald-500/40 bg-emerald-500/5 text-emerald-700 dark:text-emerald-300",
                )}
                title={isOff ? "Parked — click to re-enable" : "Click to park"}
              >
                <Power className="h-3 w-3" />
                {isOff ? "off" : "on"}
              </button>

              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-7 w-7 text-muted-foreground hover:text-destructive"
                onClick={() => removeAt(idx)}
                title="Remove target"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Row 2: target picker — provider+model for real, or
                virtual-model name for virtual */}
            {isReal ? (
              <div className="grid grid-cols-5 gap-2">
                <div className="col-span-3">
                  <ProviderPicker
                    value={row.provider ?? ""}
                    onChange={(name) => update(idx, { provider: name })}
                    providers={providers}
                    placeholder="Pick provider…"
                    size="sm"
                  />
                </div>
                <Input
                  value={row.target_model}
                  onChange={(e) =>
                    update(idx, { target_model: e.target.value })
                  }
                  placeholder="target model"
                  className="col-span-2 h-8 font-mono text-xs"
                  required
                />
              </div>
            ) : (
              <VirtualModelPicker
                value={row.target_model}
                onChange={(name) => update(idx, { target_model: name })}
                virtualModels={virtualModels}
                placeholder="Pick virtual model…"
                excludeNames={virtualExclude}
                size="sm"
              />
            )}

            {/* Row 3: weight + share bar. Disabled rows show their
                weight struck through so the operator can see what the
                row would contribute if re-enabled. */}
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
                <span>weight</span>
                <Input
                  type="number"
                  min={1}
                  value={row.weight}
                  onChange={(e) =>
                    update(idx, {
                      weight: Math.max(1, Number(e.target.value) || 1),
                    })
                  }
                  className="h-7 w-14 text-center text-xs"
                />
              </div>
              <div className="flex flex-1 items-center gap-2">
                <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-border">
                  <div
                    className={cn(
                      "h-full rounded-full transition-all",
                      isOff ? "bg-muted-foreground/30" : "bg-primary/70",
                    )}
                    style={{ width: `${share}%` }}
                  />
                </div>
                <span
                  className={cn(
                    "w-10 text-right text-[10px] font-semibold tabular-nums",
                    isOff ? "text-muted-foreground line-through" : "text-foreground",
                  )}
                >
                  {share.toFixed(0)}%
                </span>
              </div>
            </div>
          </div>
        );
      })}

      <Button
        type="button"
        variant="outline"
        size="sm"
        className="h-8 gap-1"
        onClick={append}
      >
        <Plus className="h-3 w-3" />
        Add target
      </Button>
    </div>
  );
}
