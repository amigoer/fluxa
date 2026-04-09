import { useEffect, useMemo, useState } from "react";
import {
  Plus,
  Trash2,
  Pencil,
  Search,
  AlertTriangle,
  ArrowRight,
  Shuffle,
  GitBranch,
  LogIn,
  LogOut,
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
  Routes,
  Providers,
  VirtualModels,
  type Route,
  type Provider,
  type VirtualModel,
  type VirtualModelRoute,
} from "@/lib/api";
import { useT } from "@/lib/i18n";
import { ProviderIcon } from "@/components/provider-icon";
import { ProviderPicker } from "@/components/provider-picker";
import { FallbackChainEditor } from "@/components/fallback-chain-editor";
import { WeightedTargetsEditor } from "@/components/weighted-targets-editor";
import { ConfirmDialog } from "@/components/RouteGraph/panels/ConfirmDialog";
import { cn } from "@/lib/utils";

// RoutesPage is the unified entry for both routing modes the gateway
// supports:
//
//   - fallback mode (backed by /admin/routes): a primary provider plus
//     an ordered fallback chain. The gateway tries each in sequence
//     until one succeeds.
//
//   - split mode (backed by /admin/virtual-models): a list of
//     weighted targets, each picked randomly in proportion to its
//     weight. The gateway resolver checks virtual models first, so
//     split rules take precedence over fallback rules for the same
//     name.
//
// Presenting both in one table gives operators a single mental model
// ("a route for gpt-4o") regardless of which backend table the row
// lives in. The dialog toggles between the two shapes and calls the
// correct API on save/delete.

type RouteMode = "fallback" | "split";

// UnifiedRow is the row shape used by the table. It normalises the
// two backend types so rendering logic can stay shape-agnostic.
type UnifiedRow =
  | {
      mode: "fallback";
      model: string;
      route: Route;
    }
  | {
      mode: "split";
      model: string;
      vm: VirtualModel;
    };

type FormState = {
  mode: RouteMode;
  formMode: "create" | "edit";
  // Shared
  model: string;
  description: string;
  // Fallback-mode fields
  provider: string;
  fallback: string[];
  // Split-mode fields
  targets: VirtualModelRoute[];
};

const EMPTY_FORM: FormState = {
  mode: "fallback",
  formMode: "create",
  model: "",
  description: "",
  provider: "",
  fallback: [],
  targets: [],
};

export function RoutesPage() {
  const { t } = useT();
  const [routes, setRoutes] = useState<Route[]>([]);
  const [vmodels, setVmodels] = useState<VirtualModel[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<UnifiedRow | null>(null);

  async function load() {
    try {
      // Load all three endpoints in parallel — they're independent on
      // the backend and the page needs every one before it can
      // meaningfully render either table row or form.
      const [r, vm, p] = await Promise.all([
        Routes.list(),
        VirtualModels.list(),
        Providers.list(),
      ]);
      setRoutes(r);
      setVmodels(vm);
      setProviders(p);
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

  const knownModels = useMemo(() => {
    const set = new Set<string>();
    for (const p of providers) {
      for (const m of p.models ?? []) set.add(m);
    }
    return Array.from(set).sort();
  }, [providers]);

  // Merge both tables into a single row list. Split rules take
  // precedence in the resolver, so we list them first when the same
  // model name exists in both. We also surface the collision visually
  // via a `conflict` flag below when rendering.
  const unifiedRows = useMemo<UnifiedRow[]>(() => {
    const rows: UnifiedRow[] = [];
    for (const vm of vmodels) {
      rows.push({ mode: "split", model: vm.name, vm });
    }
    for (const r of routes) {
      rows.push({ mode: "fallback", model: r.model, route: r });
    }
    return rows.sort((a, b) => a.model.localeCompare(b.model));
  }, [routes, vmodels]);

  const conflictModels = useMemo(() => {
    const seen = new Map<string, number>();
    for (const row of unifiedRows) {
      seen.set(row.model, (seen.get(row.model) ?? 0) + 1);
    }
    return new Set(
      Array.from(seen.entries())
        .filter(([, n]) => n > 1)
        .map(([k]) => k),
    );
  }, [unifiedRows]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return unifiedRows;
    return unifiedRows.filter((row) => {
      if (row.model.toLowerCase().includes(q)) return true;
      if (row.mode === "fallback") {
        if (row.route.provider.toLowerCase().includes(q)) return true;
        return (row.route.fallback ?? []).some((f) =>
          f.toLowerCase().includes(q),
        );
      }
      return row.vm.routes.some((t) =>
        (t.provider ?? "").toLowerCase().includes(q),
      );
    });
  }, [unifiedRows, query]);

  async function save() {
    if (!form) return;
    setSaving(true);
    setError(null);
    try {
      if (form.mode === "fallback") {
        await Routes.upsert({
          model: form.model.trim(),
          provider: form.provider.trim(),
          fallback: form.fallback.filter(Boolean),
        });
      } else {
        await VirtualModels.upsert({
          name: form.model.trim(),
          description: form.description.trim() || undefined,
          enabled: true,
          routes: form.targets
            // Discard rows where the operator never picked a provider.
            .filter((r) => (r.provider ?? "").trim())
            .map((r, i) => ({
              ...r,
              position: i,
              weight: Math.max(1, r.weight || 1),
              target_type: "real",
              // Empty target_model means "forward the requested model
              // name unchanged"; the backend accepts both.
              target_model: r.target_model?.trim() || form.model.trim(),
            })),
        });
      }
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function removeRow(row: UnifiedRow) {
    try {
      if (row.mode === "fallback") {
        await Routes.delete(row.model);
      } else {
        await VirtualModels.delete(row.model);
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.deleteFailed"));
    }
  }

  function openCreate(mode: RouteMode) {
    setForm({ ...EMPTY_FORM, mode });
  }

  function openEdit(row: UnifiedRow) {
    if (row.mode === "fallback") {
      setForm({
        ...EMPTY_FORM,
        mode: "fallback",
        formMode: "edit",
        model: row.model,
        provider: row.route.provider,
        fallback: row.route.fallback ?? [],
      });
    } else {
      setForm({
        ...EMPTY_FORM,
        mode: "split",
        formMode: "edit",
        model: row.model,
        description: row.vm.description ?? "",
        targets: row.vm.routes.map((r) => ({ ...r })),
      });
    }
  }

  const formValid = useMemo(() => {
    if (!form) return false;
    if (!form.model.trim()) return false;
    if (form.mode === "fallback") return !!form.provider.trim();
    return (
      form.targets.length > 0 &&
      form.targets.every((t) => (t.provider ?? "").trim())
    );
  }, [form]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("routes.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("routes.subtitle")}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button onClick={() => openCreate("fallback")}>
            <Plus className="h-4 w-4" /> {t("routes.new")}
          </Button>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {unifiedRows.length > 5 && (
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("routes.searchPlaceholder")}
            className="pl-9"
          />
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-28">{t("routes.colMode")}</TableHead>
                <TableHead>{t("routes.colModel")}</TableHead>
                <TableHead>{t("routes.colTargets")}</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredRows.map((row) => {
                const hasDangling = rowHasDangling(row, providerNames);
                const conflict = conflictModels.has(row.model);
                return (
                  <TableRow key={`${row.mode}-${row.model}`}>
                    <TableCell className="align-middle">
                      <ModePill mode={row.mode} t={t} />
                    </TableCell>
                    <TableCell className="align-middle">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-sm font-medium">
                          {row.model}
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
                        {conflict && (
                          <Badge
                            variant="outline"
                            className="gap-1 border-amber-500/40 bg-amber-500/5 text-amber-700 dark:text-amber-400"
                            title={t("routes.conflictWarning")}
                          >
                            <AlertTriangle className="h-3 w-3" />
                            {t("routes.conflictLabel")}
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="align-middle">
                      {row.mode === "fallback" ? (
                        <FallbackSummary row={row} providers={providers} />
                      ) : (
                        <SplitSummary row={row} providers={providers} />
                      )}
                    </TableCell>
                    <TableCell className="text-right align-middle">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("routes.editAction")}
                          onClick={() => openEdit(row)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("common.delete")}
                          onClick={() => setConfirmDelete(row)}
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
                    {query ? t("routes.emptySearch") : t("routes.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Create / edit dialog */}
      <Dialog
        open={form !== null}
        onOpenChange={(open) => {
          if (!open) setForm(null);
        }}
      >
        <DialogContent className="!flex max-h-[90vh] max-w-xl !flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>
              {form?.formMode === "edit" ? t("routes.edit") : t("routes.new")}
            </DialogTitle>
            <DialogDescription>
              {form?.formMode === "edit"
                ? form?.mode === "split"
                  ? t("routes.splitSubtitle")
                  : t("routes.fallbackSubtitle")
                : t("routes.newSubtitle")}
            </DialogDescription>
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
                {/* Mode selector — shown on create only. Editing a
                    row cannot switch modes because the two live in
                    different backend tables with different primary
                    keys; the operator must delete + recreate. New
                    routing strategies (canary, A/B, shadow, …) will
                    slot in as additional ModeCards here. */}
                {form.formMode === "create" && (
                  <div className="space-y-2">
                    <Label>{t("routes.fieldMode")}</Label>
                    <div className="grid grid-cols-2 gap-2">
                      <ModeCard
                        active={form.mode === "fallback"}
                        onClick={() => setForm({ ...form, mode: "fallback" })}
                        icon={<GitBranch className="h-4 w-4" />}
                        title={t("routes.modeFallback")}
                        description={t("routes.modeFallbackHint")}
                      />
                      <ModeCard
                        active={form.mode === "split"}
                        onClick={() => setForm({ ...form, mode: "split" })}
                        icon={<Shuffle className="h-4 w-4" />}
                        title={t("routes.modeSplit")}
                        description={t("routes.modeSplitHint")}
                      />
                    </div>
                  </div>
                )}

                <div className="space-y-2">
                  <Label>{t("routes.colModel")}</Label>
                  <Input
                    value={form.model}
                    onChange={(e) =>
                      setForm({ ...form, model: e.target.value })
                    }
                    placeholder="gpt-4o"
                    required
                    autoFocus={form.formMode === "create"}
                    list="fluxa-known-models"
                    disabled={form.formMode === "edit"}
                    className="font-mono"
                  />
                  <datalist id="fluxa-known-models">
                    {knownModels.map((m) => (
                      <option key={m} value={m} />
                    ))}
                  </datalist>
                  <p className="text-[11px] text-muted-foreground">
                    {t("routes.modelHint")}
                  </p>
                </div>

                {form.mode === "fallback" ? (
                  <>
                    <div className="space-y-2">
                      <Label>{t("routes.fieldPrimary")}</Label>
                      <ProviderPicker
                        value={form.provider}
                        onChange={(name) =>
                          setForm({ ...form, provider: name })
                        }
                        providers={providers}
                        placeholder={t("routes.providerPlaceholder")}
                      />
                      {form.provider &&
                        !providerNames.has(form.provider) && (
                          <p className="text-[11px] text-amber-600 dark:text-amber-400">
                            {t("routes.primaryUnknown")}
                          </p>
                        )}
                    </div>

                    <div className="space-y-2">
                      <Label>{t("routes.colFallback")}</Label>
                      <FallbackChainEditor
                        value={form.fallback}
                        onChange={(next) =>
                          setForm({ ...form, fallback: next })
                        }
                        providers={providers}
                        excludeNames={
                          form.provider ? [form.provider] : []
                        }
                      />
                      <p className="text-[11px] text-muted-foreground">
                        {t("routes.fallbackHint")}
                      </p>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="space-y-2">
                      <Label>{t("routes.fieldDescription")}</Label>
                      <Textarea
                        value={form.description}
                        onChange={(e) =>
                          setForm({ ...form, description: e.target.value })
                        }
                        rows={2}
                        placeholder={t("routes.descriptionPlaceholder")}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t("routes.fieldTargets")}</Label>
                      <WeightedTargetsEditor
                        value={form.targets}
                        onChange={(next) =>
                          setForm({ ...form, targets: next })
                        }
                        providers={providers}
                      />
                      <p className="text-[11px] text-muted-foreground">
                        {t("routes.splitHint")}
                      </p>
                    </div>
                  </>
                )}

                {/* Chain / split preview — rendered as a closed-loop
                    flow with explicit inbound/outbound endpoint nodes
                    so the operator can see the full request path from
                    client to upstream and back. */}
                {formValid && (
                  <div className="space-y-2">
                    <Label className="text-muted-foreground">
                      {t("routes.previewLabel")}
                    </Label>
                    {form.mode === "fallback" ? (
                      <FallbackFlow form={form} providers={providers} t={t} />
                    ) : (
                      <SplitFlow form={form} providers={providers} t={t} />
                    )}
                  </div>
                )}
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
        title={t("routes.deleteTitle")}
        description={
          confirmDelete
            ? t("routes.deleteConfirm", { model: confirmDelete.model })
            : ""
        }
        destructive
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        onConfirm={() => {
          if (confirmDelete) void removeRow(confirmDelete);
        }}
      />
    </div>
  );
}

// -- row helpers --------------------------------------------------------

function rowHasDangling(row: UnifiedRow, known: Set<string>): boolean {
  if (row.mode === "fallback") {
    if (!known.has(row.route.provider)) return true;
    return (row.route.fallback ?? []).some((f) => !known.has(f));
  }
  return row.vm.routes.some((r) => !!r.provider && !known.has(r.provider));
}

function ModePill({
  mode,
  t,
}: {
  mode: RouteMode;
  t: ReturnType<typeof useT>["t"];
}) {
  if (mode === "split") {
    return (
      <Badge className="gap-1 border-transparent bg-violet-500/15 text-violet-700 dark:text-violet-300">
        <Shuffle className="h-3 w-3" />
        {t("routes.modeSplit")}
      </Badge>
    );
  }
  return (
    <Badge className="gap-1 border-transparent bg-sky-500/15 text-sky-700 dark:text-sky-300">
      <GitBranch className="h-3 w-3" />
      {t("routes.modeFallback")}
    </Badge>
  );
}

function ModeCard({
  active,
  onClick,
  icon,
  title,
  description,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  title: string;
  description: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex flex-col items-start gap-1 rounded-md border p-3 text-left transition-colors",
        active
          ? "border-primary bg-primary/5 text-foreground"
          : "border-border hover:bg-accent/40 text-muted-foreground",
      )}
    >
      <div className="flex items-center gap-2 text-sm font-semibold">
        {icon}
        {title}
      </div>
      <p className="text-[11px] leading-snug">{description}</p>
    </button>
  );
}

function FallbackSummary({
  row,
  providers,
}: {
  row: Extract<UnifiedRow, { mode: "fallback" }>;
  providers: Provider[];
}) {
  const primary = providers.find((p) => p.name === row.route.provider);
  return (
    <div className="flex flex-wrap items-center gap-1">
      <ProviderPill
        name={row.route.provider}
        kind={primary?.kind}
        disabled={primary?.enabled === false}
        dangling={!primary}
      />
      {(row.route.fallback ?? []).map((f, i) => {
        const fp = providers.find((p) => p.name === f);
        return (
          <div key={`${f}-${i}`} className="flex items-center gap-1">
            <ArrowRight className="h-3 w-3 text-muted-foreground" />
            <ProviderPill
              name={f}
              kind={fp?.kind}
              disabled={fp?.enabled === false}
              dangling={!fp}
              small
            />
          </div>
        );
      })}
    </div>
  );
}

function SplitSummary({
  row,
  providers,
}: {
  row: Extract<UnifiedRow, { mode: "split" }>;
  providers: Provider[];
}) {
  const total = row.vm.routes.reduce((acc, r) => acc + (r.weight || 0), 0);
  if (row.vm.routes.length === 0) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  return (
    <div className="flex flex-wrap items-center gap-1">
      {row.vm.routes.map((r, i) => {
        const fp = providers.find((p) => p.name === r.provider);
        const pct = total > 0 ? ((r.weight || 0) / total) * 100 : 0;
        return (
          <div key={i} className="flex items-center gap-1">
            <ProviderPill
              name={r.provider ?? "?"}
              kind={fp?.kind}
              disabled={fp?.enabled === false}
              dangling={!fp}
              small
            />
            <span className="rounded bg-muted px-1 text-[10px] font-medium text-muted-foreground tabular-nums">
              {pct.toFixed(0)}%
            </span>
          </div>
        );
      })}
    </div>
  );
}

// EndpointNode — the "client" boxes anchoring each flow preview on
// the left (request coming in) and the right (response going back).
// They turn a flat chain into a closed loop so operators can mentally
// trace a whole request instead of wondering "and then what?".
function EndpointNode({
  title,
  subtitle,
  variant,
  mono,
}: {
  title: string;
  subtitle: string;
  variant: "in" | "out";
  mono?: boolean;
}) {
  const Icon = variant === "in" ? LogIn : LogOut;
  const ring =
    variant === "in"
      ? "border-sky-500/40 bg-sky-500/5 text-sky-700 dark:text-sky-300"
      : "border-emerald-500/40 bg-emerald-500/5 text-emerald-700 dark:text-emerald-300";
  return (
    <div
      className={cn(
        "flex shrink-0 flex-col items-center gap-0.5 rounded-lg border-2 px-3 py-2",
        ring,
      )}
    >
      <div className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-wider opacity-80">
        <Icon className="h-2.5 w-2.5" />
        {subtitle}
      </div>
      <div
        className={cn(
          "max-w-[120px] truncate text-xs font-semibold text-foreground",
          mono && "font-mono",
        )}
      >
        {title}
      </div>
    </div>
  );
}

// FallbackFlow — linear chain from inbound client → ordered providers
// → outbound client. Dashed "fail" arrows between providers hint that
// only one runs per request; the next only kicks in if the previous
// failed.
function FallbackFlow({
  form,
  providers,
  t,
}: {
  form: FormState;
  providers: Provider[];
  t: ReturnType<typeof useT>["t"];
}) {
  const chain = [form.provider, ...(form.fallback ?? [])].filter(Boolean);
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border border-border/60 bg-muted/30 p-3">
      <EndpointNode
        title={form.model || "—"}
        subtitle={t("routes.flowInbound")}
        variant="in"
        mono
      />
      <ArrowRight className="h-3 w-3 shrink-0 text-muted-foreground" />
      {chain.map((name, i) => {
        const p = providers.find((x) => x.name === name);
        return (
          <div key={`${name}-${i}`} className="flex items-center gap-2">
            {i > 0 && (
              <div className="flex flex-col items-center">
                <span className="text-[9px] font-medium text-muted-foreground">
                  {t("routes.flowOnFail")}
                </span>
                <ArrowRight className="h-3 w-3 text-muted-foreground/60" />
              </div>
            )}
            <div className="relative">
              <span className="absolute -left-1.5 -top-1.5 z-10 flex h-4 w-4 items-center justify-center rounded-full bg-primary text-[9px] font-bold text-primary-foreground ring-2 ring-background">
                {i + 1}
              </span>
              <ProviderPill
                name={name}
                kind={p?.kind}
                disabled={p?.enabled === false}
                dangling={!p}
              />
            </div>
          </div>
        );
      })}
      <ArrowRight className="h-3 w-3 shrink-0 text-muted-foreground" />
      <EndpointNode
        title={t("routes.flowResponse")}
        subtitle={t("routes.flowOutbound")}
        variant="out"
      />
    </div>
  );
}

// SplitFlow — fan-out topology for weighted splits. Inbound on the
// left, stacked providers in the middle with curved SVG connectors
// carrying their % share, and a single outbound on the right where
// all branches converge. The % badge sits ON each curve so the share
// reads inline with the branch it applies to.
function SplitFlow({
  form,
  providers,
  t,
}: {
  form: FormState;
  providers: Provider[];
  t: ReturnType<typeof useT>["t"];
}) {
  const targets = form.targets.filter((r) => (r.provider ?? "").trim());
  const total = targets.reduce(
    (acc, r) => acc + Math.max(0, r.weight || 0),
    0,
  );
  // Tuned so two-branch layouts have enough vertical room for the
  // curves to feel like actual curves (straight lines read as "wires"
  // not "splits").
  const minHeight = Math.max(96, targets.length * 44 + 24);

  return (
    <div
      className="flex items-stretch gap-0 rounded-md border border-border/60 bg-muted/30 p-3"
      style={{ minHeight: `${minHeight}px` }}
    >
      <div className="flex items-center">
        <EndpointNode
          title={form.model || "—"}
          subtitle={t("routes.flowInbound")}
          variant="in"
          mono
        />
      </div>

      {/* Fan-out connectors. The SVG stretches to fill the flex slot;
          each curve bows from the inbound centre (y=50) to the target
          row's centre. */}
      <div className="relative min-w-[56px] flex-1">
        <svg
          className="absolute inset-0 h-full w-full text-border"
          preserveAspectRatio="none"
          viewBox="0 0 100 100"
        >
          {targets.map((_, i) => {
            const y = ((i + 0.5) / targets.length) * 100;
            return (
              <path
                key={i}
                d={`M0,50 C50,50 50,${y} 100,${y}`}
                stroke="currentColor"
                strokeWidth="1.5"
                fill="none"
                vectorEffect="non-scaling-stroke"
              />
            );
          })}
        </svg>
        <div className="absolute inset-0 flex flex-col justify-around">
          {targets.map((r, i) => {
            const pct = total > 0 ? ((r.weight || 0) / total) * 100 : 0;
            return (
              <div key={i} className="flex justify-center">
                <span className="rounded bg-background px-1.5 py-0.5 text-[10px] font-semibold tabular-nums text-muted-foreground ring-1 ring-border">
                  {pct.toFixed(0)}%
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Target column */}
      <div className="flex flex-col justify-around gap-2">
        {targets.map((r, i) => {
          const fp = providers.find((p) => p.name === r.provider);
          return (
            <ProviderPill
              key={i}
              name={r.provider ?? "?"}
              kind={fp?.kind}
              disabled={fp?.enabled === false}
              dangling={!fp}
            />
          );
        })}
      </div>

      {/* Merge connectors — mirror of the fan-out, closing the loop. */}
      <div className="relative min-w-[56px] flex-1">
        <svg
          className="absolute inset-0 h-full w-full text-border"
          preserveAspectRatio="none"
          viewBox="0 0 100 100"
        >
          {targets.map((_, i) => {
            const y = ((i + 0.5) / targets.length) * 100;
            return (
              <path
                key={i}
                d={`M0,${y} C50,${y} 50,50 100,50`}
                stroke="currentColor"
                strokeWidth="1.5"
                fill="none"
                vectorEffect="non-scaling-stroke"
              />
            );
          })}
        </svg>
      </div>

      <div className="flex items-center">
        <EndpointNode
          title={t("routes.flowResponse")}
          subtitle={t("routes.flowOutbound")}
          variant="out"
        />
      </div>
    </div>
  );
}

function ProviderPill({
  name,
  kind,
  disabled,
  dangling,
  small,
}: {
  name: string;
  kind?: string;
  disabled?: boolean;
  dangling?: boolean;
  small?: boolean;
}) {
  const iconSize = small ? "h-3 w-3" : "h-3.5 w-3.5";
  const padding = small ? "px-1.5 py-0.5 text-[11px]" : "px-2 py-1 text-xs";
  if (dangling) {
    return (
      <div
        className={`inline-flex items-center gap-1 rounded-md border border-amber-500/40 bg-amber-500/5 font-medium text-amber-700 dark:text-amber-400 ${padding}`}
        title="Provider not found"
      >
        <AlertTriangle className={iconSize} />
        <span className="truncate">{name}</span>
      </div>
    );
  }
  return (
    <div
      className={`inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background font-medium ${padding} ${disabled ? "opacity-60" : ""}`}
      title={kind ? `${name} (${kind})` : name}
    >
      {kind && <ProviderIcon kind={kind} className={iconSize} />}
      <span className="truncate">{name}</span>
      {disabled && (
        <span className="rounded bg-muted px-1 text-[9px] text-muted-foreground">
          off
        </span>
      )}
    </div>
  );
}
