import { useEffect, useState } from "react";
import { Plus, Trash2, Pencil } from "lucide-react";
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
import { Providers, type Provider } from "@/lib/api";
import { useT } from "@/lib/i18n";

// FormState is the in-flight provider draft. We carry the structured
// fields plus a few "text-buffer" siblings (modelsText, headersText,
// deploymentsText) so the user can freely type into multi-line inputs
// without us round-tripping through JSON on every keystroke. The
// `mode` distinguishes create from edit so we can lock the primary key
// (name) and skip overwriting an unchanged secret.
type FormState = {
  mode: "create" | "edit";
  name: string;
  kind: string;
  api_key: string;
  base_url: string;
  api_version: string;
  region: string;
  access_key: string;
  secret_key: string;
  session_token: string;
  timeout_sec: number;
  enabled: boolean;
  modelsText: string;
  headersText: string;
  deploymentsText: string;
};

const EMPTY_FORM: FormState = {
  mode: "create",
  name: "",
  kind: "openai",
  api_key: "",
  base_url: "",
  api_version: "",
  region: "",
  access_key: "",
  secret_key: "",
  session_token: "",
  timeout_sec: 0,
  enabled: true,
  modelsText: "",
  headersText: "",
  deploymentsText: "",
};

// ProvidersPage manages the gateway's upstream provider entries. Every
// mutation hits POST /admin/providers which already triggers a router
// reload backend-side, so the only thing this page has to do after a
// write is refetch the list. The same dialog backs both "new" and
// "edit": flipping `form.mode` swaps the title, locks the name input,
// and treats blank secret fields as "do not overwrite".
export function ProvidersPage() {
  const { t } = useT();
  const [rows, setRows] = useState<Provider[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setRows(await Providers.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function openCreate() {
    setForm({ ...EMPTY_FORM });
  }

  function openEdit(p: Provider) {
    setForm({
      mode: "edit",
      name: p.name,
      kind: p.kind,
      // Secret fields start blank in edit mode — the backend never
      // returns the plaintext, and forcing the user to retype it on
      // every minor edit would be hostile. We treat blank as "keep
      // existing" in save() below.
      api_key: "",
      base_url: p.base_url ?? "",
      api_version: p.api_version ?? "",
      region: p.region ?? "",
      access_key: "",
      secret_key: "",
      session_token: "",
      timeout_sec: p.timeout_sec ?? 0,
      enabled: p.enabled ?? true,
      modelsText: (p.models ?? []).join(", "),
      headersText: serializeHeaders(p.headers),
      deploymentsText: serializeDeployments(p.deployments),
    });
  }

  async function save() {
    if (!form) return;
    setSaving(true);
    setError(null);
    try {
      const payload: Provider = {
        name: form.name.trim(),
        kind: form.kind.trim(),
        base_url: form.base_url.trim() || undefined,
        api_version: form.api_version.trim() || undefined,
        region: form.region.trim() || undefined,
        timeout_sec: form.timeout_sec || undefined,
        enabled: form.enabled,
        models: splitList(form.modelsText),
        headers: parseHeaders(form.headersText),
        deployments: parseDeployments(form.deploymentsText),
      };
      // Secrets: in create mode we always send what the user typed
      // (even blank for "no key needed", e.g. local Ollama). In edit
      // mode we only send a field if the user typed something — that
      // way blank means "keep the existing secret" rather than "wipe
      // it".
      if (form.mode === "create" || form.api_key) payload.api_key = form.api_key;
      if (form.mode === "create" || form.access_key) payload.access_key = form.access_key;
      if (form.mode === "create" || form.secret_key) payload.secret_key = form.secret_key;
      if (form.mode === "create" || form.session_token)
        payload.session_token = form.session_token;

      await Providers.upsert(payload);
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(name: string) {
    if (!confirm(t("providers.deleteConfirm", { name }))) return;
    try {
      await Providers.delete(name);
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
            {t("providers.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("providers.subtitle")}
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4" /> {t("providers.new")}
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
                <TableHead>{t("providers.colName")}</TableHead>
                <TableHead>{t("providers.colKind")}</TableHead>
                <TableHead>{t("providers.colBaseURL")}</TableHead>
                <TableHead>{t("providers.colStatus")}</TableHead>
                <TableHead className="w-20"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((p) => (
                <TableRow key={p.name}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>{p.kind}</TableCell>
                  <TableCell className="text-muted-foreground font-mono text-xs">
                    {p.base_url || "—"}
                  </TableCell>
                  <TableCell>
                    <Badge variant={p.enabled ? "success" : "muted"}>
                      {p.enabled
                        ? t("providers.statusEnabled")
                        : t("providers.statusDisabled")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("providers.editAction")}
                        onClick={() => openEdit(p)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        title={t("common.delete")}
                        onClick={() => remove(p.name)}
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
                    {t("providers.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Provider create/edit dialog. Wider than the default modal
          because the form spans connection + advanced + cloud-specific
          sections. Scrolls inside the modal on short viewports. */}
      <Dialog
        open={form !== null}
        onOpenChange={(open) => {
          if (!open) setForm(null);
        }}
      >
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {form?.mode === "edit"
                ? t("providers.edit")
                : t("providers.new")}
            </DialogTitle>
            <DialogDescription>{t("providers.subtitle")}</DialogDescription>
          </DialogHeader>
          {form && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void save();
              }}
              className="space-y-5"
            >
              <Section title={t("providers.sectionConnection")}>
                <div className="grid grid-cols-2 gap-4">
                  <Field label={t("providers.fieldName")}>
                    <Input
                      value={form.name}
                      onChange={(e) =>
                        setForm({ ...form, name: e.target.value })
                      }
                      required
                      autoFocus={form.mode === "create"}
                      // Name is the primary key so we lock it on edit —
                      // changing it would be a delete + recreate, which
                      // breaks any routes that point at the old name.
                      disabled={form.mode === "edit"}
                    />
                  </Field>
                  <Field label={t("providers.fieldKind")}>
                    <Input
                      value={form.kind}
                      onChange={(e) =>
                        setForm({ ...form, kind: e.target.value })
                      }
                      placeholder="openai | anthropic | azure | bedrock | gemini | deepseek | ..."
                      required
                    />
                  </Field>
                </div>
                <Field label={t("providers.fieldAPIKey")}>
                  <Input
                    type="password"
                    value={form.api_key}
                    onChange={(e) =>
                      setForm({ ...form, api_key: e.target.value })
                    }
                    placeholder={
                      form.mode === "edit"
                        ? t("providers.keyUnchanged")
                        : undefined
                    }
                  />
                </Field>
                <Field label={t("providers.fieldBaseURL")}>
                  <Input
                    value={form.base_url}
                    onChange={(e) =>
                      setForm({ ...form, base_url: e.target.value })
                    }
                    placeholder="https://api.openai.com/v1"
                  />
                </Field>
                <div className="flex items-center gap-2">
                  <input
                    id="enabled"
                    type="checkbox"
                    className="h-4 w-4 rounded border-input"
                    checked={form.enabled}
                    onChange={(e) =>
                      setForm({ ...form, enabled: e.target.checked })
                    }
                  />
                  <Label htmlFor="enabled" className="cursor-pointer">
                    {t("providers.fieldEnabled")}
                  </Label>
                </div>
              </Section>

              <Section title={t("providers.sectionAzure")}>
                <Field label={t("providers.fieldAPIVersion")}>
                  <Input
                    value={form.api_version}
                    onChange={(e) =>
                      setForm({ ...form, api_version: e.target.value })
                    }
                    placeholder="2024-02-15-preview"
                  />
                </Field>
                <Field
                  label={t("providers.fieldDeployments")}
                  hint={t("providers.fieldDeploymentsHint")}
                >
                  <Textarea
                    value={form.deploymentsText}
                    onChange={(e) =>
                      setForm({ ...form, deploymentsText: e.target.value })
                    }
                    placeholder={"gpt-4o=my-gpt4o-deployment\ngpt-4o-mini=my-mini"}
                    rows={3}
                  />
                </Field>
              </Section>

              <Section title={t("providers.sectionBedrock")}>
                <Field label={t("providers.fieldRegion")}>
                  <Input
                    value={form.region}
                    onChange={(e) =>
                      setForm({ ...form, region: e.target.value })
                    }
                    placeholder="us-east-1"
                  />
                </Field>
                <div className="grid grid-cols-2 gap-4">
                  <Field label={t("providers.fieldAccessKey")}>
                    <Input
                      type="password"
                      value={form.access_key}
                      onChange={(e) =>
                        setForm({ ...form, access_key: e.target.value })
                      }
                      placeholder={
                        form.mode === "edit"
                          ? t("providers.keyUnchanged")
                          : undefined
                      }
                    />
                  </Field>
                  <Field label={t("providers.fieldSecretKey")}>
                    <Input
                      type="password"
                      value={form.secret_key}
                      onChange={(e) =>
                        setForm({ ...form, secret_key: e.target.value })
                      }
                      placeholder={
                        form.mode === "edit"
                          ? t("providers.keyUnchanged")
                          : undefined
                      }
                    />
                  </Field>
                </div>
                <Field label={t("providers.fieldSessionToken")}>
                  <Input
                    type="password"
                    value={form.session_token}
                    onChange={(e) =>
                      setForm({ ...form, session_token: e.target.value })
                    }
                    placeholder={
                      form.mode === "edit"
                        ? t("providers.keyUnchanged")
                        : undefined
                    }
                  />
                </Field>
              </Section>

              <Section title={t("providers.sectionAdvanced")}>
                <Field
                  label={t("providers.fieldModels")}
                  hint={t("providers.fieldModelsHint")}
                >
                  <Input
                    value={form.modelsText}
                    onChange={(e) =>
                      setForm({ ...form, modelsText: e.target.value })
                    }
                    placeholder="gpt-4o, gpt-4o-mini"
                  />
                </Field>
                <Field
                  label={t("providers.fieldHeaders")}
                  hint={t("providers.fieldHeadersHint")}
                >
                  <Textarea
                    value={form.headersText}
                    onChange={(e) =>
                      setForm({ ...form, headersText: e.target.value })
                    }
                    placeholder={"X-Org-Id: 123\nX-Env: prod"}
                    rows={3}
                  />
                </Field>
                <Field label={t("providers.fieldTimeout")}>
                  <Input
                    type="number"
                    min={0}
                    value={form.timeout_sec}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        timeout_sec: Number(e.target.value) || 0,
                      })
                    }
                    placeholder="0 = default"
                  />
                </Field>
              </Section>

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

// Section is a tiny visual grouping primitive used inside the form to
// chunk the cloud-specific bits (Azure, Bedrock) under their own
// labels. It is purely cosmetic — every field still maps directly to
// providerDTO regardless of which section it lives under.
function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-3">
      <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {title}
      </div>
      <div className="space-y-3 rounded-md border border-border/60 p-4">
        {children}
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  );
}

// -- text-buffer helpers ------------------------------------------------
//
// Headers and Azure deployments are map[string]string on the wire but
// the dashboard edits them as one-line-per-entry text blocks. These
// helpers convert in both directions, ignoring blank lines and
// trimming whitespace so a stray newline never produces a phantom
// "" header.

function splitList(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function parseHeaders(text: string): Record<string, string> | undefined {
  const out: Record<string, string> = {};
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const i = trimmed.indexOf(":");
    if (i < 0) continue;
    const k = trimmed.slice(0, i).trim();
    const v = trimmed.slice(i + 1).trim();
    if (k) out[k] = v;
  }
  return Object.keys(out).length === 0 ? undefined : out;
}

function serializeHeaders(h?: Record<string, string>): string {
  if (!h) return "";
  return Object.entries(h)
    .map(([k, v]) => `${k}: ${v}`)
    .join("\n");
}

function parseDeployments(text: string): Record<string, string> | undefined {
  const out: Record<string, string> = {};
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const i = trimmed.indexOf("=");
    if (i < 0) continue;
    const k = trimmed.slice(0, i).trim();
    const v = trimmed.slice(i + 1).trim();
    if (k) out[k] = v;
  }
  return Object.keys(out).length === 0 ? undefined : out;
}

function serializeDeployments(d?: Record<string, string>): string {
  if (!d) return "";
  return Object.entries(d)
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
}
