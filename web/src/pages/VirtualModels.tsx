import { useEffect, useState } from "react";
import { Plus, Trash2, Pencil, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  VirtualModels,
  type VirtualModel,
  type VirtualModelRoute,
} from "@/lib/api";
import { useT } from "@/lib/i18n";

// VirtualModelsPage exposes the v2.4 alias / weighted-fanout feature.
// One dialog backs both create and edit; the routes table inside the
// dialog is editable inline so the operator can drag-replace the whole
// targets list in a single PUT (the backend uses full-replace upsert
// semantics by name).
type FormState = {
  mode: "create" | "edit";
  name: string;
  description: string;
  enabled: boolean;
  routes: VirtualModelRoute[];
};

const emptyRoute = (): VirtualModelRoute => ({
  weight: 1,
  target_type: "real",
  target_model: "",
  provider: "",
  enabled: true,
});

export function VirtualModelsPage() {
  const { t } = useT();
  const [rows, setRows] = useState<VirtualModel[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setRows(await VirtualModels.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save() {
    if (!form) return;
    if (form.routes.length === 0) {
      setError(t("vmodels.routesEmpty"));
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
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(name: string) {
    if (!confirm(t("vmodels.deleteConfirm", { name }))) return;
    try {
      await VirtualModels.delete(name);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.deleteFailed"));
    }
  }

  function patchRoute(idx: number, patch: Partial<VirtualModelRoute>) {
    if (!form) return;
    const routes = form.routes.map((r, i) => (i === idx ? { ...r, ...patch } : r));
    setForm({ ...form, routes });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("vmodels.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("vmodels.subtitle")}</p>
        </div>
        <Button
          onClick={() =>
            setForm({
              mode: "create",
              name: "",
              description: "",
              enabled: true,
              routes: [emptyRoute()],
            })
          }
        >
          <Plus className="h-4 w-4" /> {t("vmodels.new")}
        </Button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("vmodels.colName")}</TableHead>
                <TableHead>{t("vmodels.colDescription")}</TableHead>
                <TableHead>{t("vmodels.colRoutes")}</TableHead>
                <TableHead>{t("vmodels.colStatus")}</TableHead>
                <TableHead className="w-10"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((vm) => (
                <TableRow key={vm.name}>
                  <TableCell className="font-medium">{vm.name}</TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {vm.description || "—"}
                  </TableCell>
                  <TableCell className="text-xs">
                    {vm.routes
                      .map(
                        (r) =>
                          `${r.target_model}${
                            r.target_type === "real" && r.provider
                              ? "@" + r.provider
                              : ""
                          } (${r.weight})`,
                      )
                      .join(", ")}
                  </TableCell>
                  <TableCell>
                    <Badge variant={vm.enabled ? "success" : "muted"}>
                      {vm.enabled
                        ? t("providers.statusEnabled")
                        : t("providers.statusDisabled")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("common.edit")}
                        onClick={() =>
                          setForm({
                            mode: "edit",
                            name: vm.name,
                            description: vm.description ?? "",
                            enabled: vm.enabled ?? true,
                            routes: vm.routes.map((r) => ({ ...r })),
                          })
                        }
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("common.delete")}
                        onClick={() => remove(vm.name)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-center text-muted-foreground py-8"
                  >
                    {t("vmodels.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog
        open={form !== null}
        onOpenChange={(open) => {
          if (!open) setForm(null);
        }}
      >
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {form?.mode === "edit"
                ? t("vmodels.edit")
                : t("vmodels.new")}
            </DialogTitle>
            <DialogDescription>{t("vmodels.subtitle")}</DialogDescription>
          </DialogHeader>
          {form && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void save();
              }}
              className="space-y-4"
            >
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>{t("vmodels.fieldName")}</Label>
                  <Input
                    value={form.name}
                    onChange={(e) =>
                      setForm({ ...form, name: e.target.value })
                    }
                    placeholder="qwen-latest"
                    required
                    disabled={form.mode === "edit"}
                    autoFocus={form.mode === "create"}
                  />
                </div>
                <div className="space-y-2">
                  <Label>{t("vmodels.fieldDescription")}</Label>
                  <Input
                    value={form.description}
                    onChange={(e) =>
                      setForm({ ...form, description: e.target.value })
                    }
                  />
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(e) =>
                    setForm({ ...form, enabled: e.target.checked })
                  }
                />
                {t("vmodels.fieldEnabled")}
              </label>

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label>{t("vmodels.routesTitle")}</Label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      setForm({
                        ...form,
                        routes: [...form.routes, emptyRoute()],
                      })
                    }
                  >
                    <Plus className="h-3 w-3" /> {t("vmodels.addRoute")}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  {t("vmodels.routesHint")}
                </p>
                <div className="space-y-2">
                  {form.routes.map((r, idx) => (
                    <div
                      key={idx}
                      className="grid grid-cols-12 gap-2 items-end rounded-md border border-border/60 p-3"
                    >
                      <div className="col-span-2 space-y-1">
                        <Label className="text-xs">
                          {t("vmodels.routeWeight")}
                        </Label>
                        <Input
                          type="number"
                          min={1}
                          value={r.weight}
                          onChange={(e) =>
                            patchRoute(idx, {
                              weight: Number(e.target.value),
                            })
                          }
                        />
                      </div>
                      <div className="col-span-2 space-y-1">
                        <Label className="text-xs">
                          {t("vmodels.routeType")}
                        </Label>
                        <select
                          className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                          value={r.target_type}
                          onChange={(e) =>
                            patchRoute(idx, {
                              target_type: e.target.value as
                                | "real"
                                | "virtual",
                            })
                          }
                        >
                          <option value="real">
                            {t("vmodels.routeReal")}
                          </option>
                          <option value="virtual">
                            {t("vmodels.routeVirtual")}
                          </option>
                        </select>
                      </div>
                      <div className="col-span-4 space-y-1">
                        <Label className="text-xs">
                          {t("vmodels.routeTarget")}
                        </Label>
                        <Input
                          value={r.target_model}
                          onChange={(e) =>
                            patchRoute(idx, { target_model: e.target.value })
                          }
                          placeholder="qwen3-72b"
                          required
                        />
                      </div>
                      <div className="col-span-3 space-y-1">
                        <Label className="text-xs">
                          {t("vmodels.routeProvider")}
                        </Label>
                        <Input
                          value={r.provider ?? ""}
                          onChange={(e) =>
                            patchRoute(idx, { provider: e.target.value })
                          }
                          placeholder="qwen"
                          disabled={r.target_type !== "real"}
                          required={r.target_type === "real"}
                        />
                      </div>
                      <div className="col-span-1 flex justify-end">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          title={t("vmodels.routeRemove")}
                          onClick={() =>
                            setForm({
                              ...form,
                              routes: form.routes.filter((_, i) => i !== idx),
                            })
                          }
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <DialogFooter>
                <DialogClose asChild>
                  <Button type="button" variant="outline">
                    {t("common.cancel")}
                  </Button>
                </DialogClose>
                <Button type="submit" disabled={saving}>
                  {saving ? t("common.saving") : t("common.save")}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
