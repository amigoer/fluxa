import { useEffect, useMemo, useState } from "react";
import {
  Plus,
  Trash2,
  Pencil,
  Search,
  Power,
  Check,
  X,
  FlaskConical,
  Shield,
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
  DLPRules,
  DLPViolations,
  type DLPRule,
  type DLPViolation,
} from "@/lib/api";
import { useT } from "@/lib/i18n";
import { toast } from "@/components/ui/sonner";
import { ConfirmDialog } from "@/components/RouteGraph/panels/ConfirmDialog";
import { cn } from "@/lib/utils";

// DLPPage — two-tab view for managing data-loss-prevention rules and
// inspecting the violations log. The Rules tab mirrors the
// priority-ordered table pattern from RegexModelsPage; the Violations
// tab is a paginated read-only log with optional rule filtering.

type FormState = {
  mode: "create" | "edit";
  id?: string;
  name: string;
  pattern: string;
  pattern_type: "keyword" | "regex";
  scope: "request" | "response" | "both";
  action: "block" | "mask" | "log";
  priority: number;
  model_pattern: string;
  description: string;
  enabled: boolean;
};

const EMPTY_FORM: FormState = {
  mode: "create",
  name: "",
  pattern: "",
  pattern_type: "keyword",
  scope: "both",
  action: "block",
  priority: 100,
  model_pattern: "",
  description: "",
  enabled: true,
};

const PAGE_SIZE = 20;

const scopeColor: Record<string, string> = {
  request: "bg-sky-100 text-sky-700",
  response: "bg-amber-100 text-amber-700",
  both: "bg-violet-100 text-violet-700",
};

const actionColor: Record<string, string> = {
  block: "bg-rose-100 text-rose-700",
  mask: "bg-amber-100 text-amber-700",
  log: "bg-sky-100 text-sky-700",
};

const directionColor: Record<string, string> = {
  request: "bg-sky-100 text-sky-700",
  response: "bg-amber-100 text-amber-700",
};

export function DLPPage() {
  const { t } = useT();
  const [activeTab, setActiveTab] = useState<"rules" | "violations">("rules");

  // -- rules state --
  const [rows, setRows] = useState<DLPRule[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<DLPRule | null>(null);
  const [probe, setProbe] = useState("");

  // -- violations state --
  const [violations, setViolations] = useState<DLPViolation[]>([]);
  const [violationsTotal, setViolationsTotal] = useState(0);
  const [violationsPage, setViolationsPage] = useState(0);
  const [violationsFilter, setViolationsFilter] = useState("");

  async function loadRules() {
    try {
      const r = await DLPRules.list();
      setRows(r);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }

  async function loadViolations() {
    try {
      const params: { limit: number; offset: number; rule_id?: string } = {
        limit: PAGE_SIZE,
        offset: violationsPage * PAGE_SIZE,
      };
      if (violationsFilter) params.rule_id = violationsFilter;
      const res = await DLPViolations.list(params);
      setViolations(res.data ?? []);
      setViolationsTotal(res.total ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }

  useEffect(() => {
    void Promise.all([loadRules(), loadViolations()]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (activeTab === "violations") void loadViolations();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [violationsPage, violationsFilter, activeTab]);

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
      if (r.name.toLowerCase().includes(q)) return true;
      if (r.pattern.toLowerCase().includes(q)) return true;
      if ((r.description ?? "").toLowerCase().includes(q)) return true;
      return false;
    });
  }, [sortedRows, query]);

  const compiled = useMemo(() => {
    if (!form?.pattern) return { ok: false, error: undefined as string | undefined };
    if (form.pattern_type === "keyword") return { ok: true };
    try {
      new RegExp(form.pattern);
      return { ok: true };
    } catch (err) {
      return {
        ok: false,
        error: err instanceof Error ? err.message : "Invalid",
      };
    }
  }, [form?.pattern, form?.pattern_type]);

  const formValid = useMemo(() => {
    if (!form) return false;
    if (!form.name.trim()) return false;
    if (!form.pattern.trim()) return false;
    if (!compiled.ok) return false;
    return true;
  }, [form, compiled]);

  async function save() {
    if (!form) return;
    setSaving(true);
    setError(null);
    const payload: DLPRule = {
      name: form.name.trim(),
      pattern: form.pattern.trim(),
      pattern_type: form.pattern_type,
      scope: form.scope,
      action: form.action,
      priority: Number(form.priority) || 100,
      model_pattern: form.model_pattern.trim() || undefined,
      description: form.description.trim() || undefined,
      enabled: form.enabled,
    };
    try {
      if (form.mode === "edit" && form.id) {
        await DLPRules.update(form.id, payload);
      } else {
        await DLPRules.create(payload);
      }
      setForm(null);
      await loadRules();
      toast.success(t("common.saveSuccess"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(row: DLPRule) {
    try {
      await DLPRules.delete(row.id!);
      await loadRules();
      toast.success(t("common.deleteSuccess"));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("common.deleteFailed"),
      );
    }
  }

  function openCreate() {
    setProbe("");
    setForm({ ...EMPTY_FORM });
  }

  function openEdit(r: DLPRule) {
    setProbe("");
    setForm({
      mode: "edit",
      id: r.id,
      name: r.name,
      pattern: r.pattern,
      pattern_type: r.pattern_type,
      scope: r.scope,
      action: r.action,
      priority: r.priority,
      model_pattern: r.model_pattern ?? "",
      description: r.description ?? "",
      enabled: r.enabled ?? true,
    });
  }

  // Pattern tester result for the dialog
  function testerResult() {
    if (!form?.pattern) return null;
    if (!probe) {
      return (
        <p className="text-[11px] text-muted-foreground">
          Type test text above to check for matches.
        </p>
      );
    }
    if (form.pattern_type === "keyword") {
      const match = probe.toLowerCase().includes(form.pattern.toLowerCase());
      return match ? (
        <div className="flex items-center gap-1.5 text-[11px]">
          <Check className="h-3 w-3 text-emerald-500" />
          <span className="text-emerald-700">Match found</span>
        </div>
      ) : (
        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <X className="h-3 w-3 text-destructive" />
          <span>No match</span>
        </div>
      );
    }
    // regex mode
    if (!compiled.ok) {
      return (
        <p className="flex items-center gap-1 text-[11px] text-destructive">
          <X className="h-3 w-3" /> Invalid pattern
        </p>
      );
    }
    try {
      const re = new RegExp(form.pattern);
      const match = re.test(probe);
      return match ? (
        <div className="flex items-center gap-1.5 text-[11px]">
          <Check className="h-3 w-3 text-emerald-500" />
          <span className="text-emerald-700">Match found</span>
        </div>
      ) : (
        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <X className="h-3 w-3 text-destructive" />
          <span>No match</span>
        </div>
      );
    } catch {
      return (
        <p className="flex items-center gap-1 text-[11px] text-destructive">
          <X className="h-3 w-3" /> Invalid pattern
        </p>
      );
    }
  }

  const totalViolationPages = Math.max(1, Math.ceil(violationsTotal / PAGE_SIZE));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">
            <Shield className="mr-2 inline-block h-6 w-6 align-text-bottom" />
            DLP
          </h1>
          <p className="text-sm text-muted-foreground">
            Data loss prevention rules and violation log
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          {activeTab === "rules" && (
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" /> New Rule
            </Button>
          )}
        </div>
      </div>

      {/* Tab switcher */}
      <div className="flex gap-1 rounded-lg border border-border p-1">
        <button
          type="button"
          onClick={() => setActiveTab("rules")}
          className={cn(
            "flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors",
            activeTab === "rules"
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent",
          )}
        >
          Rules
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("violations")}
          className={cn(
            "flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors",
            activeTab === "violations"
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent",
          )}
        >
          Violations
        </button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* ====== RULES TAB ====== */}
      {activeTab === "rules" && (
        <>
          {rows.length > 5 && (
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search rules..."
                className="pl-9"
              />
            </div>
          )}

          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-12">#</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Pattern</TableHead>
                    <TableHead className="w-24">Scope</TableHead>
                    <TableHead className="w-24">Action</TableHead>
                    <TableHead className="w-24">Status</TableHead>
                    <TableHead className="w-24"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredRows.map((r, idx) => (
                    <TableRow key={r.id ?? r.name}>
                      <TableCell className="align-middle">
                        <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10 text-[11px] font-bold text-primary tabular-nums">
                          {idx + 1}
                        </span>
                      </TableCell>
                      <TableCell className="align-middle">
                        <div className="flex flex-col gap-0.5">
                          <span className="font-medium text-sm">{r.name}</span>
                          {r.description && (
                            <p className="truncate text-[11px] text-muted-foreground">
                              {r.description}
                            </p>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="align-middle">
                        <code className="rounded bg-muted/60 px-1.5 py-0.5 font-mono text-xs">
                          {r.pattern}
                        </code>
                      </TableCell>
                      <TableCell className="align-middle">
                        <Badge
                          variant="secondary"
                          className={cn(
                            "border-transparent",
                            scopeColor[r.scope],
                          )}
                        >
                          {r.scope}
                        </Badge>
                      </TableCell>
                      <TableCell className="align-middle">
                        <Badge
                          variant="secondary"
                          className={cn(
                            "border-transparent",
                            actionColor[r.action],
                          )}
                        >
                          {r.action}
                        </Badge>
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
                  ))}
                  {filteredRows.length === 0 && (
                    <TableRow>
                      <TableCell
                        colSpan={7}
                        className="py-12 text-center text-muted-foreground"
                      >
                        {query ? "No rules match your search." : "No DLP rules yet."}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      )}

      {/* ====== VIOLATIONS TAB ====== */}
      {activeTab === "violations" && (
        <>
          <div className="flex items-center gap-3">
            <select
              value={violationsFilter}
              onChange={(e) => {
                setViolationsFilter(e.target.value);
                setViolationsPage(0);
              }}
              className="h-9 rounded-md border border-border bg-background px-3 text-sm"
            >
              <option value="">All rules</option>
              {rows.map((r) => (
                <option key={r.id ?? r.name} value={r.id ?? ""}>
                  {r.name}
                </option>
              ))}
            </select>
          </div>

          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>Rule Name</TableHead>
                    <TableHead>Model</TableHead>
                    <TableHead className="w-24">Direction</TableHead>
                    <TableHead className="w-24">Action</TableHead>
                    <TableHead>Matched Text</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {violations.map((v) => (
                    <TableRow key={v.id}>
                      <TableCell className="align-middle whitespace-nowrap text-xs text-muted-foreground">
                        {formatRelativeTime(v.created_at)}
                      </TableCell>
                      <TableCell className="align-middle text-sm font-medium">
                        {v.rule_name}
                      </TableCell>
                      <TableCell className="align-middle">
                        <code className="rounded bg-muted/60 px-1.5 py-0.5 font-mono text-xs">
                          {v.model}
                        </code>
                      </TableCell>
                      <TableCell className="align-middle">
                        <Badge
                          variant="secondary"
                          className={cn(
                            "border-transparent",
                            directionColor[v.direction] ?? "",
                          )}
                        >
                          {v.direction}
                        </Badge>
                      </TableCell>
                      <TableCell className="align-middle">
                        <Badge
                          variant="secondary"
                          className={cn(
                            "border-transparent",
                            actionColor[v.action_taken] ?? "",
                          )}
                        >
                          {v.action_taken}
                        </Badge>
                      </TableCell>
                      <TableCell className="align-middle max-w-[300px]">
                        <span
                          className="block truncate font-mono text-xs"
                          title={v.matched_text}
                        >
                          {v.matched_text.length > 60
                            ? v.matched_text.slice(0, 60) + "\u2026"
                            : v.matched_text}
                        </span>
                      </TableCell>
                    </TableRow>
                  ))}
                  {violations.length === 0 && (
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        className="py-12 text-center text-muted-foreground"
                      >
                        No violations recorded.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {/* Pagination */}
          {violationsTotal > PAGE_SIZE && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {violationsTotal} violation{violationsTotal !== 1 ? "s" : ""} total
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={violationsPage === 0}
                  onClick={() => setViolationsPage((p) => Math.max(0, p - 1))}
                >
                  Previous
                </Button>
                <span className="flex items-center text-sm text-muted-foreground">
                  {violationsPage + 1} / {totalViolationPages}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={violationsPage >= totalViolationPages - 1}
                  onClick={() => setViolationsPage((p) => p + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {/* ====== CREATE / EDIT DIALOG ====== */}
      <Dialog
        open={form !== null}
        onOpenChange={(open) => {
          if (!open) setForm(null);
        }}
      >
        <DialogContent className="!flex max-h-[90vh] max-w-xl !flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>
              {form?.mode === "edit" ? "Edit Rule" : "New Rule"}
            </DialogTitle>
            <DialogDescription>
              Configure a data loss prevention rule.
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
                {/* 1. Name */}
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input
                    value={form.name}
                    onChange={(e) =>
                      setForm({ ...form, name: e.target.value })
                    }
                    placeholder="credit-card-numbers"
                    required
                    disabled={form.mode === "edit"}
                    autoFocus={form.mode === "create"}
                  />
                </div>

                {/* 2. Pattern */}
                <div className="space-y-2">
                  <Label>Pattern</Label>
                  <Input
                    value={form.pattern}
                    onChange={(e) =>
                      setForm({ ...form, pattern: e.target.value })
                    }
                    placeholder={
                      form.pattern_type === "regex"
                        ? "\\b\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}\\b"
                        : "SSN"
                    }
                    required
                    className={cn(
                      "font-mono",
                      form.pattern &&
                        !compiled.ok &&
                        "border-destructive",
                    )}
                  />
                  {form.pattern && !compiled.ok ? (
                    <p className="flex items-center gap-1 text-[11px] text-destructive">
                      <X className="h-3 w-3" />
                      {compiled.error}
                    </p>
                  ) : (
                    <p className="text-[11px] text-muted-foreground">
                      {form.pattern_type === "regex"
                        ? "JavaScript regular expression syntax"
                        : "Case-insensitive keyword match"}
                    </p>
                  )}
                </div>

                {/* 3. Pattern type */}
                <div className="space-y-2">
                  <Label>Pattern type</Label>
                  <div className="flex overflow-hidden rounded-lg border border-border">
                    {([
                      { value: "keyword" as const, label: "Keyword" },
                      { value: "regex" as const, label: "Regex" },
                    ]).map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() =>
                          setForm({ ...form, pattern_type: opt.value })
                        }
                        className={cn(
                          "flex-1 px-3 py-1.5 text-xs font-medium transition-colors",
                          form.pattern_type === opt.value
                            ? "bg-primary text-primary-foreground"
                            : "bg-background text-muted-foreground hover:bg-accent",
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* 4. Scope */}
                <div className="space-y-2">
                  <Label>Scope</Label>
                  <div className="flex overflow-hidden rounded-lg border border-border">
                    {([
                      { value: "request" as const, label: "Request" },
                      { value: "response" as const, label: "Response" },
                      { value: "both" as const, label: "Both" },
                    ]).map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() =>
                          setForm({ ...form, scope: opt.value })
                        }
                        className={cn(
                          "flex-1 px-3 py-1.5 text-xs font-medium transition-colors",
                          form.scope === opt.value
                            ? "bg-primary text-primary-foreground"
                            : "bg-background text-muted-foreground hover:bg-accent",
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* 5. Action */}
                <div className="space-y-2">
                  <Label>Action</Label>
                  <div className="flex overflow-hidden rounded-lg border border-border">
                    {([
                      { value: "block" as const, label: "Block" },
                      { value: "mask" as const, label: "Mask" },
                      { value: "log" as const, label: "Log only" },
                    ]).map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() =>
                          setForm({ ...form, action: opt.value })
                        }
                        className={cn(
                          "flex-1 px-3 py-1.5 text-xs font-medium transition-colors",
                          form.action === opt.value
                            ? "bg-primary text-primary-foreground"
                            : "bg-background text-muted-foreground hover:bg-accent",
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* 6. Priority */}
                <div className="space-y-2">
                  <Label>Priority</Label>
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

                {/* 7. Model pattern */}
                <div className="space-y-2">
                  <Label>Model pattern</Label>
                  <Input
                    value={form.model_pattern}
                    onChange={(e) =>
                      setForm({ ...form, model_pattern: e.target.value })
                    }
                    placeholder="gpt-4.*"
                    className="font-mono"
                  />
                  <p className="text-[11px] text-muted-foreground">
                    Optional regex to restrict this rule to specific models. Leave
                    blank to apply to all models.
                  </p>
                </div>

                {/* 8. Description */}
                <div className="space-y-2">
                  <Label>Description</Label>
                  <Textarea
                    value={form.description}
                    onChange={(e) =>
                      setForm({ ...form, description: e.target.value })
                    }
                    rows={2}
                    placeholder="What does this rule protect against?"
                  />
                </div>

                {/* 9. Enabled */}
                <div className="space-y-2">
                  <Label>Enabled</Label>
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

                {/* 10. Pattern tester */}
                {form.pattern && (
                  <div className="space-y-2 rounded-md border border-dashed border-border/60 bg-muted/20 p-3">
                    <div className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
                      <FlaskConical className="h-3.5 w-3.5" />
                      Pattern Tester
                    </div>
                    <Input
                      value={probe}
                      onChange={(e) => setProbe(e.target.value)}
                      placeholder="Paste text to test against the pattern..."
                      className="h-8 font-mono text-xs"
                    />
                    {testerResult()}
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
        title="Delete Rule"
        description={
          confirmDelete
            ? `Are you sure you want to delete the rule "${confirmDelete.name}"? This action cannot be undone.`
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

function formatRelativeTime(iso: string): string {
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;

  if (diff < 0) return "just now";
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(iso).toLocaleDateString();
}
