import { useEffect, useState } from "react";
import { Plus, Trash2, Copy } from "lucide-react";
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
import { Keys, type VirtualKey } from "@/lib/api";

// KeysPage manages virtual keys. Newly-created keys surface a banner
// with the full id so the operator can copy it once — the list view
// truncates the random suffix to reduce shoulder-surfing risk.
export function KeysPage() {
  const [rows, setRows] = useState<VirtualKey[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [justCreated, setJustCreated] = useState<VirtualKey | null>(null);
  const [form, setForm] = useState<
    | (Omit<VirtualKey, "models" | "ip_allowlist"> & {
        modelsText: string;
        ipText: string;
      })
    | null
  >(null);

  async function load() {
    try {
      setRows(await Keys.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : "load failed");
    }
  }
  useEffect(() => {
    load();
  }, []);

  async function save() {
    if (!form) return;
    const payload: VirtualKey = {
      name: form.name,
      description: form.description,
      models: splitList(form.modelsText),
      ip_allowlist: splitList(form.ipText),
      rpm_limit: Number(form.rpm_limit) || 0,
      budget_tokens_daily: Number(form.budget_tokens_daily) || 0,
      budget_tokens_monthly: Number(form.budget_tokens_monthly) || 0,
      budget_usd_daily: Number(form.budget_usd_daily) || 0,
      budget_usd_monthly: Number(form.budget_usd_monthly) || 0,
      enabled: true,
    };
    try {
      const created = await Keys.create(payload);
      setJustCreated(created);
      setForm(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed");
    }
  }

  async function remove(id: string) {
    if (!confirm(`Delete key ${id}? This also removes its usage history.`)) return;
    try {
      await Keys.delete(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed");
    }
  }

  async function toggle(k: VirtualKey) {
    if (!k.id) return;
    try {
      await Keys.update(k.id, { enabled: !k.enabled });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "toggle failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Virtual keys</h1>
          <p className="text-sm text-muted-foreground">
            Per-application credentials with rate limits and budgets.
          </p>
        </div>
        <Button
          onClick={() =>
            setForm({
              name: "",
              description: "",
              modelsText: "",
              ipText: "",
              rpm_limit: 0,
              budget_tokens_daily: 0,
              budget_tokens_monthly: 0,
              budget_usd_daily: 0,
              budget_usd_monthly: 0,
            })
          }
        >
          <Plus className="h-4 w-4" /> New key
        </Button>
      </div>

      {error && <div className="text-sm text-destructive">{error}</div>}

      {justCreated?.id && (
        <Card className="border-primary">
          <CardContent className="py-4 flex items-center justify-between">
            <div>
              <div className="text-sm font-medium">
                Copy this key — it will not be shown again
              </div>
              <code className="text-xs font-mono">{justCreated.id}</code>
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => navigator.clipboard.writeText(justCreated.id!)}
              >
                <Copy className="h-4 w-4" /> Copy
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setJustCreated(null)}
              >
                Dismiss
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>ID</TableHead>
                <TableHead>Models</TableHead>
                <TableHead>RPM</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-10"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((k) => (
                <TableRow key={k.id}>
                  <TableCell className="font-medium">{k.name}</TableCell>
                  <TableCell className="font-mono text-xs">
                    {k.id?.slice(0, 10)}…
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">
                    {k.models?.join(", ") || "*"}
                  </TableCell>
                  <TableCell>{k.rpm_limit || "—"}</TableCell>
                  <TableCell>
                    <button onClick={() => toggle(k)}>
                      <Badge variant={k.enabled ? "default" : "secondary"}>
                        {k.enabled ? "enabled" : "disabled"}
                      </Badge>
                    </button>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => remove(k.id!)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-muted-foreground py-8"
                  >
                    No virtual keys yet.
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
            <CardTitle>New virtual key</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input
                  value={form.description ?? ""}
                  onChange={(e) =>
                    setForm({ ...form, description: e.target.value })
                  }
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label>Models (comma-separated, * for any)</Label>
              <Input
                value={form.modelsText}
                onChange={(e) =>
                  setForm({ ...form, modelsText: e.target.value })
                }
                placeholder="gpt-4o, claude-3-5-sonnet"
              />
            </div>
            <div className="space-y-2">
              <Label>IP allowlist (CIDR or exact, comma-separated)</Label>
              <Input
                value={form.ipText}
                onChange={(e) => setForm({ ...form, ipText: e.target.value })}
                placeholder="10.0.0.0/8, 192.168.1.5"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <NumField
                label="RPM limit"
                value={form.rpm_limit}
                onChange={(v) => setForm({ ...form, rpm_limit: v })}
              />
              <NumField
                label="Daily tokens budget"
                value={form.budget_tokens_daily}
                onChange={(v) =>
                  setForm({ ...form, budget_tokens_daily: v })
                }
              />
              <NumField
                label="Monthly tokens budget"
                value={form.budget_tokens_monthly}
                onChange={(v) =>
                  setForm({ ...form, budget_tokens_monthly: v })
                }
              />
              <NumField
                label="Daily USD budget"
                value={form.budget_usd_daily}
                onChange={(v) => setForm({ ...form, budget_usd_daily: v })}
              />
              <NumField
                label="Monthly USD budget"
                value={form.budget_usd_monthly}
                onChange={(v) =>
                  setForm({ ...form, budget_usd_monthly: v })
                }
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={save}>Create</Button>
              <Button variant="outline" onClick={() => setForm(null)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function splitList(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function NumField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number | undefined;
  onChange: (v: number) => void;
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Input
        type="number"
        value={value ?? 0}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </div>
  );
}
