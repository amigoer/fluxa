import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Providers, Routes, Keys, Usage, type UsageSummary } from "@/lib/api";

// Dashboard is the landing page: counts across the fleet plus a
// fleet-wide usage summary. Intentionally lightweight — anything more
// detailed lives inside its own tab so this view stays fast.
export function DashboardPage() {
  const [counts, setCounts] = useState({ providers: 0, routes: 0, keys: 0 });
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const [p, r, k, s] = await Promise.all([
          Providers.list(),
          Routes.list(),
          Keys.list(),
          Usage.summary(),
        ]);
        setCounts({ providers: p.length, routes: r.length, keys: k.length });
        setSummary(s);
      } catch (err) {
        setError(err instanceof Error ? err.message : "load failed");
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          Fleet-wide snapshot of routing state and key usage.
        </p>
      </div>
      {error && <div className="text-sm text-destructive">{error}</div>}
      <div className="grid grid-cols-3 gap-4">
        <Stat label="Providers" value={counts.providers} />
        <Stat label="Routes" value={counts.routes} />
        <Stat label="Virtual keys" value={counts.keys} />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Usage today</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <Row label="Requests" value={summary?.daily.Requests ?? 0} />
            <Row label="Prompt tokens" value={summary?.daily.PromptTokens ?? 0} />
            <Row
              label="Completion tokens"
              value={summary?.daily.CompletionTokens ?? 0}
            />
            <Row label="Total tokens" value={summary?.daily.Tokens ?? 0} />
            <Row
              label="Cost (USD)"
              value={(summary?.daily.CostUSD ?? 0).toFixed(4)}
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Usage this month</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <Row label="Requests" value={summary?.monthly.Requests ?? 0} />
            <Row
              label="Prompt tokens"
              value={summary?.monthly.PromptTokens ?? 0}
            />
            <Row
              label="Completion tokens"
              value={summary?.monthly.CompletionTokens ?? 0}
            />
            <Row label="Total tokens" value={summary?.monthly.Tokens ?? 0} />
            <Row
              label="Cost (USD)"
              value={(summary?.monthly.CostUSD ?? 0).toFixed(4)}
            />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-muted-foreground text-xs uppercase">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold">{value}</div>
      </CardContent>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="flex justify-between">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono">{value}</span>
    </div>
  );
}
