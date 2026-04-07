import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
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

// ProvidersPage manages the gateway's upstream provider entries. Every
// mutation hits POST /admin/providers which already triggers a router
// reload backend-side, so the only thing this page has to do after a
// write is refetch the list.
export function ProvidersPage() {
  const { t } = useT();
  const [rows, setRows] = useState<Provider[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<Provider | null>(null);

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

  async function save() {
    if (!form) return;
    try {
      await Providers.upsert(form);
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
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
          <h1 className="text-2xl font-bold tracking-tight">{t("providers.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("providers.subtitle")}</p>
        </div>
        <Button
          onClick={() =>
            setForm({ name: "", kind: "openai", enabled: true, api_key: "" })
          }
        >
          <Plus className="h-4 w-4" /> {t("providers.new")}
        </Button>
      </div>

      {error && <div className="text-sm text-destructive">{error}</div>}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("providers.colName")}</TableHead>
                <TableHead>{t("providers.colKind")}</TableHead>
                <TableHead>{t("providers.colBaseURL")}</TableHead>
                <TableHead>{t("providers.colStatus")}</TableHead>
                <TableHead className="w-10"></TableHead>
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
                    <Badge variant={p.enabled ? "default" : "secondary"}>
                      {p.enabled ? t("providers.statusEnabled") : t("providers.statusDisabled")}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => remove(p.name)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
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

      {form && (
        <Card>
          <CardHeader>
            <CardTitle>{t("providers.new")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Field label={t("providers.fieldName")}>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </Field>
            <Field label={t("providers.fieldKind")}>
              <Input
                value={form.kind}
                onChange={(e) => setForm({ ...form, kind: e.target.value })}
                placeholder="openai | anthropic | azure | bedrock | gemini | deepseek | ..."
              />
            </Field>
            <Field label={t("providers.fieldAPIKey")}>
              <Input
                type="password"
                value={form.api_key ?? ""}
                onChange={(e) => setForm({ ...form, api_key: e.target.value })}
              />
            </Field>
            <Field label={t("providers.fieldBaseURL")}>
              <Input
                value={form.base_url ?? ""}
                onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                placeholder="https://api.openai.com/v1"
              />
            </Field>
            <div className="flex gap-2">
              <Button onClick={save}>{t("common.save")}</Button>
              <Button variant="outline" onClick={() => setForm(null)}>
                {t("common.cancel")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}
