import { useEffect, useState } from "react";
import { Plus, Trash2, Pencil } from "lucide-react";
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
import { RegexRoutes, type RegexRoute } from "@/lib/api";
import { useT } from "@/lib/i18n";

// RegexRoutesPage manages the priority-ordered intercept table. The
// edit dialog is the same shape for create and edit; mode flips the
// title and decides between POST (create) and PUT (update by id).
type FormState = RegexRoute & { mode: "create" | "edit" };

export function RegexRoutesPage() {
  const { t } = useT();
  const [rows, setRows] = useState<RegexRoute[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setRows(await RegexRoutes.list());
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
    setSaving(true);
    setError(null);
    const payload: RegexRoute = {
      pattern: form.pattern,
      priority: Number(form.priority) || 100,
      target_type: form.target_type,
      target_model: form.target_model,
      provider: form.target_type === "real" ? form.provider : "",
      description: form.description ?? "",
      enabled: form.enabled ?? true,
    };
    try {
      if (form.mode === "edit" && form.id) {
        await RegexRoutes.update(form.id, payload);
      } else {
        await RegexRoutes.create(payload);
      }
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(row: RegexRoute) {
    if (!confirm(t("rx.deleteConfirm"))) return;
    try {
      await RegexRoutes.delete(row.id!);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.deleteFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("rx.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("rx.subtitle")}</p>
        </div>
        <Button
          onClick={() =>
            setForm({
              mode: "create",
              pattern: "",
              priority: 100,
              target_type: "virtual",
              target_model: "",
              provider: "",
              description: "",
              enabled: true,
            })
          }
        >
          <Plus className="h-4 w-4" /> {t("rx.new")}
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
                <TableHead className="w-20">{t("rx.colPriority")}</TableHead>
                <TableHead>{t("rx.colPattern")}</TableHead>
                <TableHead>{t("rx.colTarget")}</TableHead>
                <TableHead>{t("rx.colDescription")}</TableHead>
                <TableHead>{t("rx.colStatus")}</TableHead>
                <TableHead className="w-10"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs">
                    {r.priority}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {r.pattern}
                  </TableCell>
                  <TableCell className="text-xs">
                    {r.target_type === "real"
                      ? `${r.target_model}@${r.provider}`
                      : `→ ${r.target_model}`}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {r.description || "—"}
                  </TableCell>
                  <TableCell>
                    <Badge variant={r.enabled ? "default" : "outline"}>
                      {r.enabled
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
                          setForm({ ...r, mode: "edit" })
                        }
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("common.delete")}
                        onClick={() => remove(r)}
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
                    colSpan={6}
                    className="text-center text-muted-foreground py-8"
                  >
                    {t("rx.empty")}
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {form?.mode === "edit" ? t("rx.edit") : t("rx.new")}
            </DialogTitle>
            <DialogDescription>{t("rx.subtitle")}</DialogDescription>
          </DialogHeader>
          {form && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void save();
              }}
              className="space-y-4"
            >
              <div className="space-y-2">
                <Label>{t("rx.fieldPattern")}</Label>
                <Input
                  value={form.pattern}
                  onChange={(e) =>
                    setForm({ ...form, pattern: e.target.value })
                  }
                  placeholder="^gpt-4.*"
                  required
                  autoFocus={form.mode === "create"}
                  className="font-mono"
                />
                <p className="text-xs text-muted-foreground">
                  {t("rx.fieldPatternHint")}
                </p>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>{t("rx.fieldPriority")}</Label>
                  <Input
                    type="number"
                    value={form.priority}
                    onChange={(e) =>
                      setForm({ ...form, priority: Number(e.target.value) })
                    }
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>{t("rx.fieldType")}</Label>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                    value={form.target_type}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        target_type: e.target.value as "real" | "virtual",
                      })
                    }
                  >
                    <option value="virtual">{t("vmodels.routeVirtual")}</option>
                    <option value="real">{t("vmodels.routeReal")}</option>
                  </select>
                </div>
              </div>
              <div className="space-y-2">
                <Label>{t("rx.fieldTarget")}</Label>
                <Input
                  value={form.target_model}
                  onChange={(e) =>
                    setForm({ ...form, target_model: e.target.value })
                  }
                  placeholder="qwen-latest"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>{t("rx.fieldProvider")}</Label>
                <Input
                  value={form.provider ?? ""}
                  onChange={(e) =>
                    setForm({ ...form, provider: e.target.value })
                  }
                  disabled={form.target_type !== "real"}
                  required={form.target_type === "real"}
                  placeholder="qwen"
                />
              </div>
              <div className="space-y-2">
                <Label>{t("rx.fieldDescription")}</Label>
                <Input
                  value={form.description ?? ""}
                  onChange={(e) =>
                    setForm({ ...form, description: e.target.value })
                  }
                />
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.enabled ?? true}
                  onChange={(e) =>
                    setForm({ ...form, enabled: e.target.checked })
                  }
                />
                {t("rx.fieldEnabled")}
              </label>

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
