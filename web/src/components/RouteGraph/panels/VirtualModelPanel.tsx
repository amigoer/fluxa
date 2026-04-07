// VirtualModelPanel — side-panel form for editing one virtual model
// (alias + weighted fanout). The routes table is fully editable inline
// and saved as a single full-replace upsert via POST /admin/virtual-
// models. Weight inputs do *not* auto-normalise; we surface a warning
// instead and let the operator fix it themselves so the saved
// configuration always matches what they typed.

import { useMemo, useState } from "react";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  VirtualModels,
  type VirtualModel,
  type VirtualModelRoute,
} from "@/lib/api";
import { useT } from "@/lib/i18n";

interface Props {
  model: VirtualModel;
  onChange: () => void | Promise<void>;
  onClose: () => void;
}

const emptyRoute = (): VirtualModelRoute => ({
  weight: 1,
  target_type: "real",
  target_model: "",
  provider: "",
  enabled: true,
});

export function VirtualModelPanel({ model, onChange, onClose }: Props) {
  const { t } = useT();
  const [form, setForm] = useState<VirtualModel>({
    ...model,
    routes: model.routes.map((r) => ({ ...r })),
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const totalWeight = useMemo(
    () => form.routes.reduce((acc, r) => acc + (Number(r.weight) || 0), 0),
    [form.routes],
  );

  function patchRoute(idx: number, patch: Partial<VirtualModelRoute>) {
    setForm({
      ...form,
      routes: form.routes.map((r, i) => (i === idx ? { ...r, ...patch } : r)),
    });
  }

  async function save() {
    if (form.routes.length === 0) {
      setError(t("graph.routes.empty"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await VirtualModels.upsert({
        name: form.name,
        description: form.description,
        enabled: form.enabled,
        routes: form.routes.map((r) => ({
          weight: Number(r.weight) || 1,
          target_type: r.target_type,
          target_model: r.target_model,
          provider: r.target_type === "real" ? r.provider : "",
          enabled: r.enabled ?? true,
        })),
      });
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("graph.errors.save"));
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    if (!confirm(t("graph.confirm.deleteVirtual", { name: form.name }))) return;
    setSaving(true);
    setError(null);
    try {
      await VirtualModels.delete(form.name);
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("graph.errors.delete"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">{t("graph.panel.virtualTitle")}</h3>
        <p className="text-xs text-muted-foreground">
          {t("graph.panel.virtualSubtitle")}
        </p>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.name")}</Label>
        <Input value={form.name} disabled />
      </div>

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.description")}</Label>
        <Textarea
          value={form.description ?? ""}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          rows={2}
        />
      </div>

      <label className="flex items-center gap-2 text-xs">
        <input
          type="checkbox"
          checked={form.enabled ?? true}
          onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
        />
        {t("graph.field.enabled")}
      </label>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-xs">{t("graph.field.routes")}</Label>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              setForm({ ...form, routes: [...form.routes, emptyRoute()] })
            }
          >
            <Plus className="h-3 w-3" /> {t("graph.field.add")}
          </Button>
        </div>

        {totalWeight !== 100 && totalWeight > 0 && (
          <div className="text-[10px] text-amber-600 dark:text-amber-400">
            {t("graph.weights.warning", { total: totalWeight })}
          </div>
        )}

        <div className="space-y-2">
          {form.routes.map((r, idx) => (
            <div
              key={idx}
              className="rounded-md border border-border/60 p-2 space-y-2"
            >
              <div className="flex gap-2">
                <Input
                  type="number"
                  min={1}
                  value={r.weight}
                  onChange={(e) =>
                    patchRoute(idx, { weight: Number(e.target.value) })
                  }
                  className="w-16"
                  placeholder={t("graph.field.weightPlaceholder")}
                />
                <select
                  className="flex h-9 flex-1 rounded-md border border-input bg-transparent px-2 text-xs shadow-sm"
                  value={r.target_type}
                  onChange={(e) =>
                    patchRoute(idx, {
                      target_type: e.target.value as "real" | "virtual",
                    })
                  }
                >
                  <option value="real">{t("graph.targetType.real")}</option>
                  <option value="virtual">{t("graph.targetType.virtual")}</option>
                </select>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() =>
                    setForm({
                      ...form,
                      routes: form.routes.filter((_, i) => i !== idx),
                    })
                  }
                  className="h-9 w-9 shrink-0"
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
              <Input
                value={r.target_model}
                onChange={(e) => patchRoute(idx, { target_model: e.target.value })}
                placeholder={t("graph.field.targetPlaceholder")}
                className="text-xs"
              />
              {r.target_type === "real" && (
                <Input
                  value={r.provider ?? ""}
                  onChange={(e) => patchRoute(idx, { provider: e.target.value })}
                  placeholder={t("graph.field.providerPlaceholder")}
                  className="text-xs"
                />
              )}
            </div>
          ))}
        </div>
      </div>

      <div className="flex gap-2 pt-2">
        <Button onClick={save} disabled={saving} size="sm" className="flex-1">
          {saving ? t("graph.action.saving") : t("graph.action.save")}
        </Button>
        <Button onClick={remove} disabled={saving} size="sm" variant="destructive">
          {t("graph.action.delete")}
        </Button>
      </div>
    </div>
  );
}
