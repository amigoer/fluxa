import { useEffect, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Usage, type UsageRecord } from "@/lib/api";
import { useT } from "@/lib/i18n";

// UsagePage lists the most recent usage_records rows, with an optional
// filter by virtual key id. The built-in table stays usable up to the
// 1000-row cap enforced on the server; heavier analytics live outside
// the admin surface.
export function UsagePage() {
  const { t } = useT();
  const [rows, setRows] = useState<UsageRecord[]>([]);
  const [keyFilter, setKeyFilter] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      setRows(await Usage.list(keyFilter || undefined, 200));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("usage.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("usage.subtitle")}</p>
      </div>
      <div className="flex items-end gap-2">
        <div className="flex-1 space-y-2">
          <Label>{t("usage.filterLabel")}</Label>
          <Input
            value={keyFilter}
            onChange={(e) => setKeyFilter(e.target.value)}
            placeholder="vk-…"
          />
        </div>
        <button
          onClick={load}
          className="h-9 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90"
        >
          {t("usage.refresh")}
        </button>
      </div>
      {error && <div className="text-sm text-destructive">{error}</div>}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colTime")}</TableHead>
                <TableHead>{t("usage.colKey")}</TableHead>
                <TableHead>{t("usage.colModel")}</TableHead>
                <TableHead>{t("usage.colProvider")}</TableHead>
                <TableHead className="text-right">{t("usage.colPrompt")}</TableHead>
                <TableHead className="text-right">{t("usage.colCompletion")}</TableHead>
                <TableHead className="text-right">{t("usage.colTotal")}</TableHead>
                <TableHead className="text-right">{t("usage.colUSD")}</TableHead>
                <TableHead className="text-right">{t("usage.colLatency")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((u) => (
                <TableRow key={u.ID}>
                  <TableCell className="text-xs">
                    {new Date(u.Ts).toLocaleString()}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {u.VirtualKeyID.slice(0, 10)}…
                  </TableCell>
                  <TableCell>{u.Model}</TableCell>
                  <TableCell>{u.Provider}</TableCell>
                  <TableCell className="text-right font-mono">
                    {u.PromptTokens}
                  </TableCell>
                  <TableCell className="text-right font-mono">
                    {u.CompletionTokens}
                  </TableCell>
                  <TableCell className="text-right font-mono">
                    {u.TotalTokens}
                  </TableCell>
                  <TableCell className="text-right font-mono">
                    {u.CostUSD.toFixed(5)}
                  </TableCell>
                  <TableCell className="text-right text-muted-foreground">
                    {u.LatencyMs}ms
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className="text-center text-muted-foreground py-8"
                  >
                    {t("usage.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
