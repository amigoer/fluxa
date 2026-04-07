// RegexRoutePanel — side-panel form for editing one regex intercept
// rule. Save calls PUT /admin/regex-routes/:id; Delete calls DELETE.
// Both trigger an onChange callback so the parent can re-fetch and
// rebuild the graph after a successful mutation.

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { RegexRoutes, type RegexRoute } from "@/lib/api";
import { useT } from "@/lib/i18n";

interface Props {
  // route + onUpdate together describe two modes:
  //
  //   1. Edit mode  : route has an id, onUpdate is undefined.
  //                   The form keeps its own local state and only
  //                   touches the server on Save (PUT).
  //
  //   2. Create mode: route has no id (it's the in-progress draft
  //                   sitting on the canvas), onUpdate is the
  //                   "write through to the canvas" hook. The form
  //                   becomes a *controlled* component — every
  //                   keystroke flows through onUpdate so the
  //                   canvas card visually mirrors the typed data.
  //                   On Save we POST a brand-new row.
  //
  // We branch on `route?.id` rather than the presence of route so a
  // draft route with no id correctly enters create mode.
  route?: RegexRoute;
  onUpdate?: (route: RegexRoute) => void;
  onChange: () => void | Promise<void>;
  onClose: () => void;
}

const EMPTY_REGEX_ROUTE: RegexRoute = {
  pattern: "",
  priority: 100,
  target_type: "virtual",
  target_model: "",
  provider: "",
  description: "",
  enabled: true,
};

export function RegexRoutePanel({
  route,
  onUpdate,
  onChange,
  onClose,
}: Props) {
  const { t } = useT();
  const isCreate = !route?.id;
  const isControlled = !!onUpdate;
  // Local fallback used only in edit mode. In create mode the parent
  // owns the source of truth (and re-renders us when drag-to-connect
  // mutates the draft), so we read from `route` directly.
  const [localForm, setLocalForm] = useState<RegexRoute>(
    route ?? EMPTY_REGEX_ROUTE,
  );
  const form: RegexRoute = isControlled
    ? (route ?? EMPTY_REGEX_ROUTE)
    : localForm;
  const setForm = (next: RegexRoute) => {
    if (isControlled) onUpdate!(next);
    else setLocalForm(next);
  };
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const payload: RegexRoute = {
        pattern: form.pattern,
        priority: Number(form.priority) || 100,
        target_type: form.target_type,
        target_model: form.target_model,
        provider: form.target_type === "real" ? form.provider : "",
        description: form.description ?? "",
        enabled: form.enabled ?? true,
      };
      if (isCreate) {
        await RegexRoutes.create(payload);
      } else if (form.id) {
        await RegexRoutes.update(form.id, payload);
      } else {
        return;
      }
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("graph.errors.save"));
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    if (!form.id) return;
    if (!confirm(t("graph.confirm.deleteRegex"))) return;
    setSaving(true);
    setError(null);
    try {
      await RegexRoutes.delete(form.id);
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
        <Label className="text-xs">{t("graph.field.pattern")}</Label>
        <Input
          value={form.pattern}
          onChange={(e) => setForm({ ...form, pattern: e.target.value })}
          className="font-mono text-xs"
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label className="text-xs">{t("graph.field.priority")}</Label>
          <Input
            type="number"
            value={form.priority}
            onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label className="text-xs">{t("graph.field.targetType")}</Label>
          <select
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm"
            value={form.target_type}
            onChange={(e) =>
              setForm({ ...form, target_type: e.target.value as "real" | "virtual" })
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
          value={form.target_model}
          onChange={(e) => setForm({ ...form, target_model: e.target.value })}
        />
      </div>

      {form.target_type === "real" && (
        <div className="space-y-2">
          <Label className="text-xs">{t("graph.field.provider")}</Label>
          <Input
            value={form.provider ?? ""}
            onChange={(e) => setForm({ ...form, provider: e.target.value })}
          />
        </div>
      )}

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
