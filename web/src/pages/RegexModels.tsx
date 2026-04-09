import { useEffect, useMemo, useState } from "react";
import {
  Plus,
  Trash2,
  Pencil,
  Search,
  AlertTriangle,
  Box,
  Layers,
  Power,
  Check,
  X,
  ArrowRight,
  FlaskConical,
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
  RegexModels,
  Providers,
  VirtualModels,
  type RegexModel,
  type Provider,
  type VirtualModel,
} from "@/lib/api";
import { useT } from "@/lib/i18n";
import { ProviderPicker } from "@/components/provider-picker";
import { VirtualModelPicker } from "@/components/virtual-model-picker";
import { ProviderPill } from "@/components/provider-pill";
import { toast } from "@/components/ui/sonner";
import { ConfirmDialog } from "@/components/RouteGraph/panels/ConfirmDialog";
import { cn } from "@/lib/utils";

// RegexModelsPage — priority-ordered intercept table. The gateway
// runs each enabled pattern in ascending priority order and the
// first match wins; everything below it on the list is skipped. That
// first-match-wins semantics is THE thing the UI has to communicate
// clearly, so the table is always sorted by effective priority and
// each row carries an explicit order badge (1, 2, 3…). Raw priority
// numbers stay editable because operators sometimes want to leave
// gaps (e.g. priority 10, 20, 30) so they can insert new rules
// without renumbering the neighbours.
//
// A live regex tester sits inside the form so operators can type a
// model name and immediately see whether the pattern matches plus
// the rewrite it would emit. That feedback loop is the single most
// useful thing we can offer for a page whose central mistake is
// "pattern didn't match what I thought it would".

type FormState = {
  mode: "create" | "edit";
  id?: string;
  pattern: string;
  priority: number;
  target_type: "real" | "virtual";
  target_model: string;
  provider: string;
  description: string;
  enabled: boolean;
};

const EMPTY_FORM: FormState = {
  mode: "create",
  pattern: "",
  priority: 100,
  target_type: "virtual",
  target_model: "",
  provider: "",
  description: "",
  enabled: true,
};

export function RegexModelsPage() {
  const { t } = useT();
  const [rows, setRows] = useState<RegexModel[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [vmodels, setVmodels] = useState<VirtualModel[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<RegexModel | null>(null);
  const [probe, setProbe] = useState("");

  async function load() {
    try {
      const [r, p, v] = await Promise.all([
        RegexModels.list(),
        Providers.list(),
        VirtualModels.list(),
      ]);
      setRows(r);
      setProviders(p);
      setVmodels(v);
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
  const vmNames = useMemo(() => new Set(vmodels.map((v) => v.name)), [vmodels]);

  // Always render rows in the order the resolver actually evaluates
  // them. Stable tie-break on created_at keeps the table from
  // jittering when two rules share a priority.
  const sortedRows = useMemo(() => {
    return [...rows].sort((a, b) => {
      if (a.priority !== b.priority) return a.priority - b.priority;
      return (a.created_at ?? "").localeCompare(b.created_at ?? "");
    });
  }, [rows]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sortedRows;
    return sortedRows.filter((r) => {
      if (r.pattern.toLowerCase().includes(q)) return true;
      if (r.target_model.toLowerCase().includes(q)) return true;
      if ((r.provider ?? "").toLowerCase().includes(q)) return true;
      if ((r.description ?? "").toLowerCase().includes(q)) return true;
      return false;
    });
  }, [sortedRows, query]);

  async function save() {
    if (!form) return;
    setSaving(true);
    setError(null);
    const payload: RegexModel = {
      pattern: form.pattern.trim(),
      priority: Number(form.priority) || 100,
      target_type: form.target_type,
      target_model: form.target_model.trim(),
      provider:
        form.target_type === "real" ? form.provider.trim() : "",
      description: form.description.trim() || undefined,
      enabled: form.enabled,
    };
    try {
      if (form.mode === "edit" && form.id) {
        await RegexModels.update(form.id, payload);
      } else {
        await RegexModels.create(payload);
      }
      setForm(null);
      await load();
      toast.success(t("common.saveSuccess"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(row: RegexModel) {
    try {
      await RegexModels.delete(row.id!);
      await load();
      toast.success(t("common.deleteSuccess"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("common.deleteFailed"));
    }
  }

  function openCreate() {
    setProbe("");
    setForm({ ...EMPTY_FORM });
  }

  function openEdit(r: RegexModel) {
    setProbe("");
    setForm({
      mode: "edit",
      id: r.id,
      pattern: r.pattern,
      priority: r.priority,
      target_type: r.target_type,
      target_model: r.target_model,
      provider: r.provider ?? "",
      description: r.description ?? "",
      enabled: r.enabled ?? true,
    });
  }

  // Live regex compile check. Swallows the SyntaxError and hands the
  // callsite a nice {ok, error?} so form validation can gate the
  // save button without crashing on every keystroke.
  const compiled = useMemo(() => {
    if (!form?.pattern) return { ok: false, error: undefined as string | undefined };
    try {
      return { ok: true, regex: new RegExp(form.pattern) };
    } catch (err) {
      return {
        ok: false,
        error: err instanceof Error ? err.message : "Invalid pattern",
      };
    }
  }, [form?.pattern]);

  const formValid = useMemo(() => {
    if (!form) return false;
    if (!form.pattern.trim() || !compiled.ok) return false;
    if (!form.target_model.trim()) return false;
    if (form.target_type === "real" && !form.provider.trim()) return false;
    return true;
  }, [form, compiled]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("rx.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("rx.subtitle")}</p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" /> {t("rx.new")}
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
            placeholder={t("rx.searchPlaceholder")}
            className="pl-9"
          />
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-16">{t("rx.colOrder")}</TableHead>
                <TableHead>{t("rx.colPattern")}</TableHead>
                <TableHead>{t("rx.colTarget")}</TableHead>
                <TableHead className="w-28">{t("rx.colStatus")}</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredRows.map((r, idx) => {
                const dangling = isDangling(r, providerNames, vmNames);
                const patternOk = tryCompile(r.pattern);
                return (
                  <TableRow key={r.id}>
                    <TableCell className="align-middle">
                      <div className="flex items-center gap-1.5">
                        <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10 text-[11px] font-bold text-primary tabular-nums">
                          {idx + 1}
                        </span>
                        <span className="font-mono text-[10px] text-muted-foreground">
                          p{r.priority}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="align-middle">
                      <div className="flex flex-col gap-0.5">
                        <div className="flex items-center gap-1.5">
                          <code
                            className={cn(
                              "rounded bg-muted/60 px-1.5 py-0.5 font-mono text-xs",
                              !patternOk && "text-destructive",
                            )}
                          >
                            {r.pattern}
                          </code>
                          {!patternOk && (
                            <AlertTriangle
                              className="h-3.5 w-3.5 text-destructive"
                              aria-label="Invalid regex"
                            />
                          )}
                        </div>
                        {r.description && (
                          <p className="truncate text-[11px] text-muted-foreground">
                            {r.description}
                          </p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="align-middle">
                      <TargetDisplay
                        row={r}
                        providers={providers}
                        dangling={dangling}
                      />
                    </TableCell>
                    <TableCell className="align-middle">
                      <Badge variant={r.enabled ? "success" : "muted"}>
                        {r.enabled
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
                          onClick={() => openEdit(r)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("common.delete")}
                          onClick={() => setConfirmDelete(r)}
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
                    colSpan={5}
                    className="py-12 text-center text-muted-foreground"
                  >
                    {query ? t("rx.emptySearch") : t("rx.empty")}
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
              {form?.mode === "edit" ? t("rx.edit") : t("rx.new")}
            </DialogTitle>
            <DialogDescription>{t("rx.subtitle")}</DialogDescription>
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
                    className={cn(
                      "font-mono",
                      form.pattern && !compiled.ok && "border-destructive",
                    )}
                  />
                  {form.pattern && !compiled.ok ? (
                    <p className="flex items-center gap-1 text-[11px] text-destructive">
                      <AlertTriangle className="h-3 w-3" />
                      {compiled.error}
                    </p>
                  ) : (
                    <p className="text-[11px] text-muted-foreground">
                      {t("rx.fieldPatternHint")}
                    </p>
                  )}
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>{t("rx.fieldPriority")}</Label>
                    <Input
                      type="number"
                      value={form.priority}
                      onChange={(e) =>
                        setForm({
                          ...form,
                          priority: Number(e.target.value) || 0,
                        })
                      }
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t("rx.fieldEnabled")}</Label>
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

                {/* Target type selector — same segmented-button
                    language as VirtualTargetsEditor so all routing
                    pages share the visual vocabulary. */}
                <div className="space-y-2">
                  <Label>{t("rx.fieldType")}</Label>
                  <div className="flex overflow-hidden rounded-md border border-border/60">
                    <button
                      type="button"
                      onClick={() =>
                        setForm({
                          ...form,
                          target_type: "real",
                          target_model: "",
                          provider: "",
                        })
                      }
                      className={cn(
                        "flex flex-1 items-center justify-center gap-2 px-3 py-2 text-sm font-medium transition-colors",
                        form.target_type === "real"
                          ? "bg-primary text-primary-foreground"
                          : "bg-background text-muted-foreground hover:bg-accent",
                      )}
                    >
                      <Box className="h-3.5 w-3.5" />
                      {t("vmodels.routeReal")}
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        setForm({
                          ...form,
                          target_type: "virtual",
                          target_model: "",
                          provider: "",
                        })
                      }
                      className={cn(
                        "flex flex-1 items-center justify-center gap-2 border-l border-border/60 px-3 py-2 text-sm font-medium transition-colors",
                        form.target_type === "virtual"
                          ? "bg-primary text-primary-foreground"
                          : "bg-background text-muted-foreground hover:bg-accent",
                      )}
                    >
                      <Layers className="h-3.5 w-3.5" />
                      {t("vmodels.routeVirtual")}
                    </button>
                  </div>
                </div>

                {form.target_type === "real" ? (
                  <div className="space-y-3">
                    <div className="space-y-2">
                      <Label>{t("rx.fieldProvider")}</Label>
                      <ProviderPicker
                        value={form.provider}
                        onChange={(name) =>
                          setForm({ ...form, provider: name })
                        }
                        providers={providers}
                        placeholder={t("routes.providerPlaceholder")}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t("rx.fieldTarget")}</Label>
                      <Input
                        value={form.target_model}
                        onChange={(e) =>
                          setForm({ ...form, target_model: e.target.value })
                        }
                        placeholder="gpt-4o-2024-08-06"
                        required
                        className="font-mono"
                      />
                    </div>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <Label>{t("rx.fieldTarget")}</Label>
                    <VirtualModelPicker
                      value={form.target_model}
                      onChange={(name) =>
                        setForm({ ...form, target_model: name })
                      }
                      virtualModels={vmodels}
                      placeholder="Pick virtual model…"
                    />
                  </div>
                )}

                <div className="space-y-2">
                  <Label>{t("rx.fieldDescription")}</Label>
                  <Textarea
                    value={form.description}
                    onChange={(e) =>
                      setForm({ ...form, description: e.target.value })
                    }
                    rows={2}
                    placeholder={t("rx.descriptionPlaceholder")}
                  />
                </div>

                {/* Live pattern tester — the single highest-leverage
                    affordance on this page. Type a sample model name
                    and immediately see whether the current pattern
                    would match plus the rewrite it would emit. */}
                <div className="space-y-2 rounded-md border border-dashed border-border/60 bg-muted/20 p-3">
                  <div className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
                    <FlaskConical className="h-3.5 w-3.5" />
                    {t("rx.testerLabel")}
                  </div>
                  <Input
                    value={probe}
                    onChange={(e) => setProbe(e.target.value)}
                    placeholder={t("rx.testerPlaceholder")}
                    className="h-8 font-mono text-xs"
                  />
                  <TesterResult
                    probe={probe}
                    form={form}
                    compiled={compiled}
                    providers={providers}
                    t={t}
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
        title={t("rx.deleteTitle")}
        description={confirmDelete ? t("rx.deleteConfirm") : ""}
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

function tryCompile(pattern: string): boolean {
  try {
    new RegExp(pattern);
    return true;
  } catch {
    return false;
  }
}

function isDangling(
  r: RegexModel,
  providerNames: Set<string>,
  vmNames: Set<string>,
): boolean {
  if (r.target_type === "real") {
    return !!r.provider && !providerNames.has(r.provider);
  }
  return !!r.target_model && !vmNames.has(r.target_model);
}

function TargetDisplay({
  row,
  providers,
  dangling,
}: {
  row: RegexModel;
  providers: Provider[];
  dangling: boolean;
}) {
  if (row.target_type === "virtual") {
    return (
      <div className="flex items-center gap-1.5">
        <div
          className={cn(
            "inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium",
            dangling
              ? "border-amber-500/40 bg-amber-500/5 text-amber-700 dark:text-amber-400"
              : "border-violet-500/40 bg-violet-500/5 text-violet-700 dark:text-violet-300",
          )}
          title={
            dangling
              ? `Virtual model "${row.target_model}" not found`
              : `Virtual model: ${row.target_model}`
          }
        >
          {dangling ? (
            <AlertTriangle className="h-3 w-3" />
          ) : (
            <Layers className="h-3 w-3" />
          )}
          <span className="truncate">{row.target_model}</span>
        </div>
      </div>
    );
  }
  const p = providers.find((x) => x.name === row.provider);
  return (
    <div className="flex items-center gap-1.5">
      <ProviderPill
        name={row.provider ?? "?"}
        kind={p?.kind}
        disabled={p?.enabled === false}
        dangling={dangling}
        small
      />
      <span className="font-mono text-[10px] text-muted-foreground">
        /{row.target_model}
      </span>
    </div>
  );
}

function TesterResult({
  probe,
  form,
  compiled,
  providers,
  t,
}: {
  probe: string;
  form: FormState;
  compiled: { ok: boolean; regex?: RegExp; error?: string };
  providers: Provider[];
  t: ReturnType<typeof useT>["t"];
}) {
  if (!probe) {
    return (
      <p className="text-[11px] text-muted-foreground">
        {t("rx.testerIdle")}
      </p>
    );
  }
  if (!compiled.ok || !compiled.regex) {
    return (
      <p className="flex items-center gap-1 text-[11px] text-destructive">
        <AlertTriangle className="h-3 w-3" /> {t("rx.testerInvalid")}
      </p>
    );
  }
  const match = compiled.regex.test(probe);
  if (!match) {
    return (
      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        <X className="h-3 w-3 text-destructive" /> {t("rx.testerNoMatch")}
      </div>
    );
  }
  const p = providers.find((x) => x.name === form.provider);
  return (
    <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
      <Check className="h-3 w-3 text-emerald-500" />
      <span className="text-muted-foreground">{t("rx.testerMatch")}</span>
      <span className="rounded bg-background px-1.5 py-0.5 font-mono text-[10px]">
        {probe}
      </span>
      <ArrowRight className="h-3 w-3 text-muted-foreground" />
      {form.target_type === "virtual" ? (
        <div className="inline-flex items-center gap-1 rounded-md border border-violet-500/40 bg-violet-500/5 px-1.5 py-0.5 font-medium text-violet-700 dark:text-violet-300">
          <Layers className="h-3 w-3" />
          {form.target_model || "—"}
        </div>
      ) : (
        <>
          <ProviderPill
            name={form.provider || "?"}
            kind={p?.kind}
            dangling={!!form.provider && !p}
            small
          />
          <span className="font-mono text-[10px] text-muted-foreground">
            /{form.target_model || "—"}
          </span>
        </>
      )}
    </div>
  );
}
