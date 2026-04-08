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
  // create  : model is the empty draft payload, name field editable,
  //           Save POSTs a new record.
  // edit    : model is the saved row, name is read-only, Save PUTs.
  //
  // We discriminate by an explicit flag rather than guessing from
  // the shape of `model` — the previous heuristic (isControlled =
  // !!onUpdate) ended up making the input's controlled value flicker
  // against the Zustand store on every keystroke and the name field
  // stopped accepting input on some renders. Local state is the
  // source of truth; the canvas draft node is a pure placeholder
  // that is replaced by the real saved node on load().
  model?: VirtualModel;
  create?: boolean;
  onChange: () => void | Promise<void>;
  onClose: () => void;
}

// emptyRoute returns a fresh weighted route with a *smart* default
// weight: whatever is needed to bring the running total up to 100. So
// the first route on a brand-new VM defaults to 100, a second route
// added to a [100] list defaults to 0 (operator must rebalance), and
// a third added to [50, 30] defaults to 20. This trains the operator
// to think of weights as percentages from the very first interaction.
const emptyRoute = (currentTotal = 0): VirtualModelRoute => ({
  weight: Math.max(0, 100 - currentTotal),
  target_type: "real",
  target_model: "",
  provider: "",
  enabled: true,
});

// EMPTY_VM seeds a brand-new virtual model with one route at 100%, so
// the initial state is already valid (sum equals 100). The operator
// just types a name and a target.
const EMPTY_VM: VirtualModel = {
  name: "",
  description: "",
  enabled: true,
  routes: [
    {
      weight: 100,
      target_type: "real",
      target_model: "",
      provider: "",
      enabled: true,
    },
  ],
};

export function VirtualModelPanel({
  model,
  create,
  onChange,
  onClose,
}: Props) {
  const { t } = useT();
  const isCreate = !!create;
  const [form, setForm] = useState<VirtualModel>(
    model
      ? { ...model, routes: model.routes.map((r) => ({ ...r })) }
      : { ...EMPTY_VM, routes: EMPTY_VM.routes.map((r) => ({ ...r })) },
  );
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
      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.name")}</Label>
        <Input
          value={form.name}
          disabled={!isCreate}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          placeholder={isCreate ? "qwen-latest" : undefined}
        />
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
              setForm({
                ...form,
                routes: [...form.routes, emptyRoute(totalWeight)],
              })
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
          {saving
            ? t("graph.action.saving")
            : isCreate
              ? t("graph.action.create")
              : t("graph.action.save")}
        </Button>
        {!isCreate && (
          <Button
            onClick={remove}
            disabled={saving}
            size="sm"
            variant="destructive"
          >
            {t("graph.action.delete")}
          </Button>
        )}
      </div>
    </div>
  );
}
