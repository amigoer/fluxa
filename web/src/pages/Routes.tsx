import { useEffect, useState } from "react";
import { Plus, Trash2, Pencil } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
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
import { Routes, type Route } from "@/lib/api";
import { useT } from "@/lib/i18n";

// RoutesPage is the model → provider map. Fallbacks are entered as a
// comma-separated list to keep the form trivial; the store itself holds
// them as a JSON array. The same dialog backs both create and edit —
// `form.mode` flips the title and locks the model name (the primary
// key) so an edit cannot accidentally orphan an existing entry.
type FormState = Route & { fallbackText: string; mode: "create" | "edit" };

export function RoutesPage() {
  const { t } = useT();
  const [rows, setRows] = useState<Route[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setRows(await Routes.list());
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
    try {
      await Routes.upsert({
        model: form.model,
        provider: form.provider,
        fallback: form.fallbackText
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(model: string) {
    if (!confirm(t("routes.deleteConfirm", { model }))) return;
    try {
      await Routes.delete(model);
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
            {t("routes.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("routes.subtitle")}</p>
        </div>
        <Button
          onClick={() =>
            setForm({
              mode: "create",
              model: "",
              provider: "",
              fallback: [],
              fallbackText: "",
            })
          }
        >
          <Plus className="h-4 w-4" /> {t("routes.new")}
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
                <TableHead>{t("routes.colModel")}</TableHead>
                <TableHead>{t("routes.colProvider")}</TableHead>
                <TableHead>{t("routes.colFallback")}</TableHead>
                <TableHead className="w-10"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.model}>
                  <TableCell className="font-medium">{r.model}</TableCell>
                  <TableCell>{r.provider}</TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {r.fallback?.join(", ") || "—"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("routes.editAction")}
                        onClick={() =>
                          setForm({
                            mode: "edit",
                            model: r.model,
                            provider: r.provider,
                            fallback: r.fallback,
                            fallbackText: (r.fallback ?? []).join(", "),
                          })
                        }
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("common.delete")}
                        onClick={() => remove(r.model)}
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
                    colSpan={4}
                    className="text-center text-muted-foreground py-8"
                  >
                    {t("routes.empty")}
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
              {form?.mode === "edit" ? t("routes.edit") : t("routes.new")}
            </DialogTitle>
            <DialogDescription>{t("routes.subtitle")}</DialogDescription>
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
                <Label>{t("routes.colModel")}</Label>
                <Input
                  value={form.model}
                  onChange={(e) =>
                    setForm({ ...form, model: e.target.value })
                  }
                  placeholder="gpt-4o"
                  required
                  autoFocus={form.mode === "create"}
                  // Model is the primary key, so we lock it on edit:
                  // changing it would be a delete + recreate.
                  disabled={form.mode === "edit"}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("routes.colProvider")}</Label>
                <Input
                  value={form.provider}
                  onChange={(e) =>
                    setForm({ ...form, provider: e.target.value })
                  }
                  placeholder="openai"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>{t("routes.fieldFallback")}</Label>
                <Input
                  value={form.fallbackText}
                  onChange={(e) =>
                    setForm({ ...form, fallbackText: e.target.value })
                  }
                  placeholder="azure, anthropic"
                />
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
