import { NavLink } from "react-router";

import { DataState, PageHeader } from "@/components/page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useResource } from "@/hooks/use-resource";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api";
import { formatNumber, formatRelative, formatUSD } from "@/lib/format";

function StatCard({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl tabular-nums">{value}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground text-xs">{hint}</p>
      </CardContent>
    </Card>
  );
}

export function OverviewPage() {
  const t = useT();
  const usage = useResource(() => api.usageSummary());
  const providers = useResource(() => api.listProviders());
  const logs = useResource(() => api.listLogs({ limit: 8 }));

  const daily = usage.data?.daily;
  const monthly = usage.data?.monthly;
  const enabledProviders = (providers.data ?? []).filter((p) => p.enabled !== false).length;

  return (
    <>
      <PageHeader title={t("overview.title")} description={t("overview.description")} />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label={t("overview.requestsToday")}
          value={formatNumber(daily?.Requests ?? 0)}
          hint={t("overview.thisMonth", { value: formatNumber(monthly?.Requests ?? 0) })}
        />
        <StatCard
          label={t("overview.tokensToday")}
          value={formatNumber(daily?.Tokens ?? 0)}
          hint={t("overview.thisMonth", { value: formatNumber(monthly?.Tokens ?? 0) })}
        />
        <StatCard
          label={t("overview.spendToday")}
          value={formatUSD(daily?.CostUSD ?? 0)}
          hint={t("overview.thisMonth", { value: formatUSD(monthly?.CostUSD ?? 0) })}
        />
        <StatCard
          label={t("overview.providers")}
          value={formatNumber(enabledProviders)}
          hint={t("overview.configured", { count: formatNumber(providers.data?.length ?? 0) })}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("overview.recentTitle")}</CardTitle>
          <CardDescription>{t("overview.recentDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <DataState
            loading={logs.loading}
            error={logs.error}
            empty={(logs.data?.data ?? []).length === 0}
            emptyMessage={t("overview.recentEmpty")}
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("overview.colWhen")}</TableHead>
                  <TableHead>{t("common.model")}</TableHead>
                  <TableHead>{t("common.provider")}</TableHead>
                  <TableHead className="text-right">{t("overview.colTokens")}</TableHead>
                  <TableHead className="text-right">{t("overview.colLatency")}</TableHead>
                  <TableHead className="text-right">{t("common.status")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(logs.data?.data ?? []).map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="text-muted-foreground whitespace-nowrap">
                      {formatRelative(log.started_at)}
                    </TableCell>
                    <TableCell className="font-medium">
                      {log.model_resolved || log.model_requested || "—"}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{log.provider || "—"}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(log.total_tokens)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{log.latency_ms} ms</TableCell>
                    <TableCell className="text-right">
                      <Badge variant={log.status_code >= 400 ? "destructive" : "secondary"}>
                        {log.status_code}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </DataState>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("overview.providersTitle")}</CardTitle>
          <CardDescription>{t("overview.providersDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <DataState
            loading={providers.loading}
            error={providers.error}
            empty={(providers.data ?? []).length === 0}
            emptyMessage={t("overview.providersEmpty")}
          >
            <div className="flex flex-wrap gap-2">
              {(providers.data ?? []).map((provider) => (
                <Badge
                  key={provider.name}
                  variant={provider.enabled === false ? "outline" : "secondary"}
                >
                  {provider.name}
                  <span className="text-muted-foreground ml-1">({provider.kind})</span>
                </Badge>
              ))}
            </div>
          </DataState>
        </CardContent>
      </Card>

      <div className="flex gap-2">
        <Button asChild variant="outline">
          <NavLink to="/providers">{t("overview.manageProviders")}</NavLink>
        </Button>
        <Button asChild variant="outline">
          <NavLink to="/logs">{t("overview.browseLogs")}</NavLink>
        </Button>
      </div>
    </>
  );
}
