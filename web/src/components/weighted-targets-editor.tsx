import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ProviderPicker } from "@/components/provider-picker";
import type { Provider, VirtualModelRoute } from "@/lib/api";
import { cn } from "@/lib/utils";

// WeightedTargetsEditor — the split-mode counterpart to the fallback
// chain editor. It manages an unordered list of {provider, weight,
// target_model?} rows and live-renders each row's share of traffic as
// a percentage derived from the weight sum.
//
// A few deliberate simplifications:
//
//  - target_type is hard-coded to "real". Recursion into other virtual
//    models is still supported via the dedicated Virtual Models page;
//    dragging that complexity into the Routes dialog would bury the
//    common case (split a request across two upstreams) under options
//    most operators never touch.
//
//  - target_model defaults to empty (= "forward the requested model
//    name unchanged"). An override field is shown as a hint-level
//    secondary input so the common case stays one-line.
//
//  - weight is a free integer >= 1. We don't force it to sum to 100
//    because the backend normalises internally — operators can think
//    in "3:1" ratios without doing arithmetic.

interface Props {
  value: VirtualModelRoute[];
  onChange: (next: VirtualModelRoute[]) => void;
  providers: Provider[];
}

export function WeightedTargetsEditor({ value, onChange, providers }: Props) {
  const total = value.reduce((acc, r) => acc + Math.max(0, r.weight || 0), 0);

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

  const excludeNames = value.map((r) => r.provider ?? "").filter(Boolean);

  return (
    <div className="space-y-2">
      {value.length === 0 && (
        <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-center text-xs text-muted-foreground">
          No targets yet. Add at least one provider to split traffic.
        </div>
      )}

      {value.map((row, idx) => {
        const share = total > 0 ? ((row.weight || 0) / total) * 100 : 0;
        return (
          <div
            key={idx}
            className="space-y-2 rounded-md border border-border/60 bg-muted/20 p-2.5"
          >
            <div className="flex items-start gap-2">
              <div className="flex-1 space-y-2">
                <ProviderPicker
                  value={row.provider ?? ""}
                  onChange={(name) => update(idx, { provider: name })}
                  providers={providers}
                  excludeNames={excludeNames.filter((n) => n !== row.provider)}
                  placeholder="Pick provider…"
                  size="sm"
                />
                <div className="flex items-center gap-2">
                  <Input
                    value={row.target_model ?? ""}
                    onChange={(e) =>
                      update(idx, { target_model: e.target.value })
                    }
                    placeholder="Override model (optional)"
                    className="h-8 flex-1 font-mono text-xs"
                  />
                </div>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1">
                <div className="flex items-center gap-1">
                  <Input
                    type="number"
                    min={1}
                    value={row.weight}
                    onChange={(e) =>
                      update(idx, {
                        weight: Math.max(1, Number(e.target.value) || 1),
                      })
                    }
                    className="h-8 w-16 text-center text-xs"
                    title="Weight"
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                    onClick={() => removeAt(idx)}
                    title="Remove target"
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
                {/* Share percentage — a tiny inline bar so the split
                    between two 1:3 weights reads as ~25% / ~75% at a
                    glance. */}
                <div className="flex items-center gap-1.5 text-[10px] tabular-nums text-muted-foreground">
                  <div className="h-1 w-16 overflow-hidden rounded-full bg-border">
                    <div
                      className={cn(
                        "h-full rounded-full transition-all",
                        share > 0 ? "bg-primary/70" : "bg-transparent",
                      )}
                      style={{ width: `${share}%` }}
                    />
                  </div>
                  <span className="w-9 text-right">{share.toFixed(0)}%</span>
                </div>
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
