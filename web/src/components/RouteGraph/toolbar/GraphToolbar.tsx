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
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface Props {
  onLayout: () => void;
  onChange: () => void | Promise<void>;
}

export function GraphToolbar({ onLayout, onChange }: Props) {
  const { t } = useT();
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const toggleLiveMode = useRouteGraphStore((s) => s.toggleLiveMode);
  const { fitView } = useReactFlow();

  const [showRegex, setShowRegex] = useState(false);
  const [showVm, setShowVm] = useState(false);

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
          onClick={() => setShowRegex(true)}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <RegexIcon className="h-3.5 w-3.5" />
          {t("graph.toolbar.regex")}
        </button>
        <button
          onClick={() => setShowVm(true)}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[12px] font-medium text-foreground hover:bg-muted transition-colors"
        >
          <GitBranch className="h-3.5 w-3.5" />
          {t("graph.toolbar.virtual")}
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
  const { t } = useT();
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
      setError(err instanceof Error ? err.message : t("graph.errors.create"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("graph.dialog.newRegex")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {error && (
            <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label className="text-xs">{t("graph.field.pattern")}</Label>
            <Input
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              placeholder={t("graph.dialog.patternPlaceholder")}
              className="font-mono"
              required
              autoFocus
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label className="text-xs">{t("graph.field.priority")}</Label>
              <Input
                type="number"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
                required
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs">{t("graph.field.targetType")}</Label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm"
                value={targetType}
                onChange={(e) =>
                  setTargetType(e.target.value as "real" | "virtual")
                }
              >
                <option value="virtual">{t("graph.targetType.virtual")}</option>
                <option value="real">{t("graph.targetType.real")}</option>
              </select>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-xs">{t("graph.field.targetModel")}</Label>
            <Input
              value={targetModel}
              onChange={(e) => setTargetModel(e.target.value)}
              required
            />
          </div>
          {targetType === "real" && (
            <div className="space-y-2">
              <Label className="text-xs">{t("graph.field.provider")}</Label>
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
                {t("graph.action.cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={saving}>
              {saving ? t("graph.action.saving") : t("graph.action.create")}
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
  const { t } = useT();
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
            // Default the only route to 100% so the new VM is
            // immediately valid; the operator can rebalance after
            // adding more targets.
            weight: 100,
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
      setError(err instanceof Error ? err.message : t("graph.errors.create"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("graph.dialog.newVirtual")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {error && (
            <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label className="text-xs">{t("graph.field.name")}</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("graph.dialog.namePlaceholder")}
              required
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs">{t("graph.field.initialTarget")}</Label>
            <Input
              value={targetModel}
              onChange={(e) => setTargetModel(e.target.value)}
              placeholder={t("graph.dialog.targetPlaceholder")}
              required
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs">{t("graph.field.provider")}</Label>
            <Input
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
              placeholder={t("graph.dialog.providerPlaceholder")}
              required
            />
          </div>
          <p className="text-[10px] text-muted-foreground">
            {t("graph.dialog.virtualHint")}
          </p>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                {t("graph.action.cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={saving}>
              {saving ? t("graph.action.saving") : t("graph.action.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
