import { useEffect, useMemo, useState } from "react";
import {
  Plus,
  Trash2,
  Pencil,
  Search,
  AlertTriangle,
  Layers,
  Power,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
  Providers,
  type VirtualModel,
  type VirtualModelRoute,
  type Provider,
} from "@/lib/api";
import { useT } from "@/lib/i18n";
import { ProviderPill } from "@/components/provider-pill";
import { VirtualTargetsEditor } from "@/components/virtual-targets-editor";
import { ConfirmDialog } from "@/components/RouteGraph/panels/ConfirmDialog";
import { cn } from "@/lib/utils";

// VirtualModelsPage is the power-user counterpart to the Routes page.
// Routes handles the common "one model name → one strategy" shape;
// this page exposes the full virtual-model compositional feature:
// weighted targets that can themselves point at other virtual models
// (recursive, capped at 5 hops by the resolver) plus per-target
// enable/disable so operators can park rows during incidents without
// losing configuration.
//
// The overlap with Routes split mode is intentional — simple splits
// work in either place; this page only earns its keep when the
// operator needs virtual-of-virtuals or wants to temporarily mute a
// target.

type FormState = {
  mode: "create" | "edit";
  name: string;
  description: string;
  enabled: boolean;
  routes: VirtualModelRoute[];
};

const EMPTY_FORM: FormState = {
  mode: "create",
  name: "",
  description: "",
  enabled: true,
  routes: [],
};

export function VirtualModelsPage() {
  const { t } = useT();
  const [rows, setRows] = useState<VirtualModel[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<VirtualModel | null>(null);

  async function load() {
    try {
      const [vms, ps] = await Promise.all([
        VirtualModels.list(),
        Providers.list(),
      ]);
      setRows(vms);
      setProviders(ps);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const providerNames = useMemo(
    () => new Set(providers.map((p) => p.name)),
    [providers],
  );
  const vmNames = useMemo(() => new Set(rows.map((v) => v.name)), [rows]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((vm) => {
      if (vm.name.toLowerCase().includes(q)) return true;
      if ((vm.description ?? "").toLowerCase().includes(q)) return true;
      return vm.routes.some((r) => {
        if ((r.provider ?? "").toLowerCase().includes(q)) return true;
        return (r.target_model ?? "").toLowerCase().includes(q);
      });
    });
  }, [rows, query]);

  async function save() {
    if (!form) return;
    if (!form.name.trim()) {
      setError(t("vmodels.routesEmpty"));
      return;
    }
    if (form.routes.length === 0) {
      setError(t("vmodels.routesEmpty"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await VirtualModels.upsert({
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        enabled: form.enabled,
        routes: form.routes.map((r, i) => ({
          weight: Math.max(1, Number(r.weight) || 1),
          target_type: r.target_type,
          target_model: r.target_model.trim(),
          provider: r.target_type === "real" ? (r.provider ?? "").trim() : "",
          enabled: r.enabled ?? true,
          position: i,
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

  async function remove(vm: VirtualModel) {
    try {
      await VirtualModels.delete(vm.name);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.deleteFailed"));
    }
  }

  function openCreate() {
    setForm({
      ...EMPTY_FORM,
      routes: [
        {
          weight: 1,
          target_type: "real",
          target_model: "",
          provider: "",
          enabled: true,
          position: 0,
        },
      ],
    });
  }

  function openEdit(vm: VirtualModel) {
    setForm({
      mode: "edit",
      name: vm.name,
      description: vm.description ?? "",
      enabled: vm.enabled ?? true,
      routes: vm.routes.map((r) => ({ ...r })),
    });
  }

  const formValid = useMemo(() => {
    if (!form) return false;
    if (!form.name.trim()) return false;
    if (form.routes.length === 0) return false;
    return form.routes.every((r) => {
      if (!r.target_model.trim()) return false;
      if (r.target_type === "real" && !(r.provider ?? "").trim()) return false;
      return true;
    });
  }, [form]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("vmodels.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("vmodels.subtitle")}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" /> {t("vmodels.new")}
          </Button>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {rows.length > 5 && (
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("vmodels.searchPlaceholder")}
            className="pl-9"
          />
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("vmodels.colName")}</TableHead>
                <TableHead>{t("vmodels.colRoutes")}</TableHead>
                <TableHead className="w-28">
                  {t("vmodels.colStatus")}
                </TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredRows.map((vm) => {
                const hasDangling = vm.routes.some((r) =>
                  rowDangling(r, providerNames, vmNames),
                );
                return (
                  <TableRow key={vm.name}>
                    <TableCell className="align-middle">
                      <div className="flex flex-col gap-0.5">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-sm font-medium">
                            {vm.name}
                          </span>
                          {hasDangling && (
                            <Badge
                              variant="outline"
                              className="gap-1 border-amber-500/40 bg-amber-500/5 text-amber-700 dark:text-amber-400"
                              title={t("routes.danglingWarning")}
                            >
                              <AlertTriangle className="h-3 w-3" />
                              {t("routes.danglingLabel")}
                            </Badge>
                          )}
                        </div>
                        {vm.description && (
                          <p className="truncate text-[11px] text-muted-foreground">
                            {vm.description}
                          </p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="align-middle">
                      <TargetsSummary
                        routes={vm.routes}
                        providers={providers}
                        vmNames={vmNames}
                      />
                    </TableCell>
                    <TableCell className="align-middle">
                      <Badge variant={vm.enabled ? "success" : "muted"}>
                        {vm.enabled
                          ? t("providers.statusEnabled")
                          : t("providers.statusDisabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right align-middle">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("common.edit")}
                          onClick={() => openEdit(vm)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("common.delete")}
                          onClick={() => setConfirmDelete(vm)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
              {filteredRows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={4}
                    className="py-12 text-center text-muted-foreground"
                  >
                    {query ? t("vmodels.emptySearch") : t("vmodels.empty")}
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
        <DialogContent className="!flex max-h-[90vh] max-w-2xl !flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>
              {form?.mode === "edit" ? t("vmodels.edit") : t("vmodels.new")}
            </DialogTitle>
            <DialogDescription>{t("vmodels.subtitle")}</DialogDescription>
          </DialogHeader>
          {form && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!formValid) return;
                void save();
              }}
              className="flex min-h-0 flex-1 flex-col"
            >
              <div className="-mx-1 flex-1 space-y-5 overflow-y-auto px-1 pb-2">
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
                      className="font-mono"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("vmodels.fieldEnabled")}</Label>
                    <button
                      type="button"
                      onClick={() =>
                        setForm({ ...form, enabled: !form.enabled })
                      }
                      className={cn(
                        "flex h-9 w-full items-center gap-2 rounded-md border px-3 text-sm font-medium transition-colors",
                        form.enabled
                          ? "border-emerald-500/40 bg-emerald-500/5 text-emerald-700 dark:text-emerald-300"
                          : "border-border bg-background text-muted-foreground",
                      )}
                    >
                      <Power className="h-4 w-4" />
                      {form.enabled
                        ? t("providers.statusEnabled")
                        : t("providers.statusDisabled")}
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>{t("vmodels.fieldDescription")}</Label>
                  <Textarea
                    value={form.description}
                    onChange={(e) =>
                      setForm({ ...form, description: e.target.value })
                    }
                    rows={2}
                    placeholder={t("vmodels.descriptionPlaceholder")}
                  />
                </div>

                <div className="space-y-2">
                  <Label>{t("vmodels.routesTitle")}</Label>
                  <p className="text-[11px] text-muted-foreground">
                    {t("vmodels.routesHint")}
                  </p>
                  <VirtualTargetsEditor
                    value={form.routes}
                    onChange={(next) => setForm({ ...form, routes: next })}
                    providers={providers}
                    virtualModels={rows}
                    selfName={form.name}
                  />
                </div>
              </div>

              <DialogFooter className="mt-4 border-t border-border/60 pt-4">
                <DialogClose asChild>
                  <Button type="button" variant="outline">
                    {t("common.cancel")}
                  </Button>
                </DialogClose>
                <Button type="submit" disabled={saving || !formValid}>
                  {saving ? t("common.saving") : t("common.save")}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirmDelete !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmDelete(null);
        }}
        title={t("vmodels.deleteTitle")}
        description={
          confirmDelete
            ? t("vmodels.deleteConfirm", { name: confirmDelete.name })
            : ""
        }
        destructive
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        onConfirm={() => {
          if (confirmDelete) void remove(confirmDelete);
        }}
      />
    </div>
  );
}

// -- helpers ---------------------------------------------------------

function rowDangling(
  r: VirtualModelRoute,
  providerNames: Set<string>,
  vmNames: Set<string>,
): boolean {
  if (r.target_type === "real") {
    return !!r.provider && !providerNames.has(r.provider);
  }
  return !!r.target_model && !vmNames.has(r.target_model);
}

function TargetsSummary({
  routes,
  providers,
  vmNames,
}: {
  routes: VirtualModelRoute[];
  providers: Provider[];
  vmNames: Set<string>;
}) {
  // Live-weight total that ignores parked rows — matches the
  // backend's picker, which also skips disabled entries.
  const total = routes.reduce(
    (acc, r) => acc + (r.enabled === false ? 0 : Math.max(0, r.weight || 0)),
    0,
  );
  if (routes.length === 0) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {routes.map((r, i) => {
        const isOff = r.enabled === false;
        const pct =
          total > 0 && !isOff ? ((r.weight || 0) / total) * 100 : 0;
        return (
          <div
            key={i}
            className={cn(
              "flex items-center gap-1",
              isOff && "opacity-50",
            )}
          >
            {r.target_type === "real" ? (
              <ProviderPill
                name={r.provider ?? "?"}
                kind={providers.find((p) => p.name === r.provider)?.kind}
                dangling={
                  !!r.provider &&
                  !providers.some((p) => p.name === r.provider)
                }
                small
              />
            ) : (
              <VirtualPill
                name={r.target_model || "?"}
                dangling={
                  !!r.target_model && !vmNames.has(r.target_model)
                }
              />
            )}
            {r.target_type === "real" && r.target_model && (
              <span className="font-mono text-[10px] text-muted-foreground">
                /{r.target_model}
              </span>
            )}
            <span
              className={cn(
                "rounded bg-muted px-1 text-[10px] font-medium tabular-nums",
                isOff
                  ? "text-muted-foreground line-through"
                  : "text-muted-foreground",
              )}
            >
              {isOff ? "off" : `${pct.toFixed(0)}%`}
            </span>
          </div>
        );
      })}
    </div>
  );
}

// VirtualPill — rendered in-page because a virtual-model reference
// needs a different visual (Layers icon, no provider kind) from a
// real-provider reference. Kept tiny and non-exported: it only
// matters inside this page.
function VirtualPill({
  name,
  dangling,
}: {
  name: string;
  dangling?: boolean;
}) {
  if (dangling) {
    return (
      <div
        className="inline-flex items-center gap-1 rounded-md border border-amber-500/40 bg-amber-500/5 px-1.5 py-0.5 text-[11px] font-medium text-amber-700 dark:text-amber-400"
        title={`Virtual model "${name}" not found`}
      >
        <AlertTriangle className="h-3 w-3" />
        <span className="truncate">{name}</span>
      </div>
    );
  }
  return (
    <div
      className="inline-flex items-center gap-1 rounded-md border border-violet-500/40 bg-violet-500/5 px-1.5 py-0.5 text-[11px] font-medium text-violet-700 dark:text-violet-300"
      title={`Virtual model: ${name}`}
    >
      <Layers className="h-3 w-3" />
      <span className="truncate">{name}</span>
    </div>
  );
}

