import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Providers, Routes, Keys, Usage, type UsageSummary } from "@/lib/api";
import { useT, type TranslationKey } from "@/lib/i18n";

// Dashboard is the landing page: counts across the fleet plus a
// fleet-wide usage summary. Intentionally lightweight — anything more
// detailed lives inside its own tab so this view stays fast.
export function DashboardPage() {
  const { t } = useT();
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
        setError(err instanceof Error ? err.message : t("common.loadFailed"));
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("dashboard.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("dashboard.subtitle")}</p>
      </div>
      {error && <div className="text-sm text-destructive">{error}</div>}
      <div className="grid grid-cols-3 gap-4">
        <Stat labelKey="dashboard.providers" value={counts.providers} />
        <Stat labelKey="dashboard.routes" value={counts.routes} />
        <Stat labelKey="dashboard.keys" value={counts.keys} />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.usageToday")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <Row labelKey="dashboard.requests" value={summary?.daily.Requests ?? 0} />
            <Row labelKey="dashboard.promptTokens" value={summary?.daily.PromptTokens ?? 0} />
            <Row labelKey="dashboard.completionTokens" value={summary?.daily.CompletionTokens ?? 0} />
            <Row labelKey="dashboard.totalTokens" value={summary?.daily.Tokens ?? 0} />
            <Row labelKey="dashboard.costUSD" value={(summary?.daily.CostUSD ?? 0).toFixed(4)} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.usageMonth")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <Row labelKey="dashboard.requests" value={summary?.monthly.Requests ?? 0} />
            <Row labelKey="dashboard.promptTokens" value={summary?.monthly.PromptTokens ?? 0} />
            <Row labelKey="dashboard.completionTokens" value={summary?.monthly.CompletionTokens ?? 0} />
            <Row labelKey="dashboard.totalTokens" value={summary?.monthly.Tokens ?? 0} />
            <Row labelKey="dashboard.costUSD" value={(summary?.monthly.CostUSD ?? 0).toFixed(4)} />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Stat({ labelKey, value }: { labelKey: TranslationKey; value: number }) {
  const { t } = useT();
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-muted-foreground text-xs uppercase">
          {t(labelKey)}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold">{value}</div>
      </CardContent>
    </Card>
  );
}

function Row({ labelKey, value }: { labelKey: TranslationKey; value: number | string }) {
  const { t } = useT();
  return (
    <div className="flex justify-between">
      <span className="text-muted-foreground">{t(labelKey)}</span>
      <span className="font-mono">{value}</span>
    </div>
  );
}
