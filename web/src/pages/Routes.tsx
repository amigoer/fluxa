import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Routes, type Route } from "@/lib/api";

// RoutesPage is the model → provider map. Fallbacks are entered as a
// comma-separated list to keep the form trivial; the store itself holds
// them as a JSON array.
export function RoutesPage() {
  const [rows, setRows] = useState<Route[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<
    (Route & { fallbackText: string }) | null
  >(null);

  async function load() {
    try {
      setRows(await Routes.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : "load failed");
    }
  }
  useEffect(() => {
    load();
  }, []);

  async function save() {
    if (!form) return;
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
      setError(err instanceof Error ? err.message : "save failed");
    }
  }

  async function remove(model: string) {
    if (!confirm(`Delete route for ${model}?`)) return;
    try {
      await Routes.delete(model);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Routes</h1>
          <p className="text-sm text-muted-foreground">
            Model-name → provider mapping with optional fallback chain.
          </p>
        </div>
        <Button
          onClick={() =>
            setForm({ model: "", provider: "", fallback: [], fallbackText: "" })
          }
        >
          <Plus className="h-4 w-4" /> New route
        </Button>
      </div>

      {error && <div className="text-sm text-destructive">{error}</div>}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Model</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Fallback</TableHead>
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
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => remove(r.model)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={4}
                    className="text-center text-muted-foreground py-8"
                  >
                    No routes yet.
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
            <CardTitle>New route</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Model</Label>
              <Input
                value={form.model}
                onChange={(e) => setForm({ ...form, model: e.target.value })}
                placeholder="gpt-4o"
              />
            </div>
            <div className="space-y-2">
              <Label>Provider</Label>
              <Input
                value={form.provider}
                onChange={(e) => setForm({ ...form, provider: e.target.value })}
                placeholder="openai"
              />
            </div>
            <div className="space-y-2">
              <Label>Fallback (comma-separated)</Label>
              <Input
                value={form.fallbackText}
                onChange={(e) =>
                  setForm({ ...form, fallbackText: e.target.value })
                }
                placeholder="azure, anthropic"
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={save}>Save</Button>
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
