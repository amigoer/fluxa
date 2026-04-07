// GraphToolbar — floating action bar pinned to the top of the canvas.
// Hosts the create-new-rule entry points, the manual layout reset, the
// live mode toggle (with a pulsing dot when active), and the React
// Flow fitView shortcut. Create flows reuse the existing dialog
// components from the RegexRoutes / VirtualModels pages by way of
// inline mini-forms — we keep them inline so the graph view stays a
// self-contained subtree without poking at sibling pages.

import { useState } from "react";
import {
  Maximize2,
  RefreshCw,
  Activity,
  GitBranch,
  Regex as RegexIcon,
} from "lucide-react";
import { useReactFlow } from "@xyflow/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import { RegexRoutes, VirtualModels } from "@/lib/api";
import { cn } from "@/lib/utils";

interface Props {
  onLayout: () => void;
  onChange: () => void | Promise<void>;
}

export function GraphToolbar({ onLayout, onChange }: Props) {
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const toggleLiveMode = useRouteGraphStore((s) => s.toggleLiveMode);
  const { fitView } = useReactFlow();

  const [showRegex, setShowRegex] = useState(false);
  const [showVm, setShowVm] = useState(false);

  return (
    <>
      <div className="absolute top-3 left-3 z-10 flex items-center gap-1.5 rounded-lg border border-border/60 bg-background/95 backdrop-blur px-2 py-1.5 shadow-sm">
        <Button size="sm" variant="ghost" onClick={() => setShowRegex(true)}>
          <RegexIcon className="h-3.5 w-3.5" /> Regex Route
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setShowVm(true)}>
          <GitBranch className="h-3.5 w-3.5" /> Virtual Model
        </Button>
        <div className="h-4 w-px bg-border" />
        <Button size="sm" variant="ghost" onClick={onLayout} title="Re-run auto layout">
          <RefreshCw className="h-3.5 w-3.5" /> Auto Layout
        </Button>
        <div className="h-4 w-px bg-border" />
        <Button
          size="sm"
          variant="ghost"
          onClick={toggleLiveMode}
          className={cn(liveMode && "text-red-500")}
        >
          <span className="relative inline-flex items-center">
            <Activity className="h-3.5 w-3.5" />
            {liveMode && (
              <span className="absolute -top-0.5 -right-0.5 h-1.5 w-1.5 rounded-full bg-red-500 animate-pulse" />
            )}
          </span>
          Live
        </Button>
        <div className="h-4 w-px bg-border" />
        <Button size="sm" variant="ghost" onClick={() => fitView({ duration: 300 })}>
          <Maximize2 className="h-3.5 w-3.5" /> Fit View
        </Button>
      </div>

      <CreateRegexDialog
        open={showRegex}
        onClose={() => setShowRegex(false)}
        onChange={onChange}
      />
      <CreateVirtualModelDialog
        open={showVm}
        onClose={() => setShowVm(false)}
        onChange={onChange}
      />
    </>
  );
}

// CreateRegexDialog is the inline mini-form behind the "+ Regex Route"
// toolbar action. Mirrors the fields of the side panel form so the
// experience is consistent — minus the delete button (nothing to
// delete on a brand-new record).
function CreateRegexDialog({
  open,
  onClose,
  onChange,
}: {
  open: boolean;
  onClose: () => void;
  onChange: () => void | Promise<void>;
}) {
  const [pattern, setPattern] = useState("");
  const [priority, setPriority] = useState(100);
  const [targetType, setTargetType] = useState<"real" | "virtual">("virtual");
  const [targetModel, setTargetModel] = useState("");
  const [provider, setProvider] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await RegexRoutes.create({
        pattern,
        priority,
        target_type: targetType,
        target_model: targetModel,
        provider: targetType === "real" ? provider : "",
        enabled: true,
      });
      // Reset form so the next open starts fresh.
      setPattern("");
      setPriority(100);
      setTargetType("virtual");
      setTargetModel("");
      setProvider("");
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New regex route</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {error && (
            <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label className="text-xs">Pattern</Label>
            <Input
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              placeholder="^gpt-4.*"
              className="font-mono"
              required
              autoFocus
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label className="text-xs">Priority</Label>
              <Input
                type="number"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
                required
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs">Target type</Label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm"
                value={targetType}
                onChange={(e) =>
                  setTargetType(e.target.value as "real" | "virtual")
                }
              >
                <option value="virtual">virtual</option>
                <option value="real">real</option>
              </select>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-xs">Target model</Label>
            <Input
              value={targetModel}
              onChange={(e) => setTargetModel(e.target.value)}
              required
            />
          </div>
          {targetType === "real" && (
            <div className="space-y-2">
              <Label className="text-xs">Provider</Label>
              <Input
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                required
              />
            </div>
          )}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={saving}>
              {saving ? "Saving…" : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// CreateVirtualModelDialog mirrors the regex create dialog but for the
// alias-with-routes case. We seed it with one empty route so the
// operator can fill in the minimum viable VM in two text inputs.
function CreateVirtualModelDialog({
  open,
  onClose,
  onChange,
}: {
  open: boolean;
  onClose: () => void;
  onChange: () => void | Promise<void>;
}) {
  const [name, setName] = useState("");
  const [targetModel, setTargetModel] = useState("");
  const [provider, setProvider] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await VirtualModels.upsert({
        name,
        enabled: true,
        routes: [
          {
            weight: 1,
            target_type: "real",
            target_model: targetModel,
            provider,
            enabled: true,
          },
        ],
      });
      setName("");
      setTargetModel("");
      setProvider("");
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New virtual model</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {error && (
            <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label className="text-xs">Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="qwen-latest"
              required
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs">Initial target model</Label>
            <Input
              value={targetModel}
              onChange={(e) => setTargetModel(e.target.value)}
              placeholder="qwen3-72b-instruct"
              required
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs">Provider</Label>
            <Input
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
              placeholder="qwen"
              required
            />
          </div>
          <p className="text-[10px] text-muted-foreground">
            Add more weighted targets later from the side panel.
          </p>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={saving}>
              {saving ? "Saving…" : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
