import { useState } from "react";
import { ActivityIcon, CoinsIcon, ServerIcon, TriangleAlertIcon } from "lucide-react";
import { NavLink } from "react-router";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { DataState, PageHeader } from "@/components/page";
import { SegmentedGroup, SegmentedItem } from "@/components/segmented";
import { ProviderLogo } from "@/components/provider-logo";
import { normalise, StatCard } from "@/components/stat-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useResource } from "@/hooks/use-resource";
import { api, type AnalyticsBreakdown } from "@/lib/api";
import { formatNumber, formatRelative, formatUSD } from "@/lib/format";
import { useT, type TranslationKey } from "@/lib/i18n";

const RANGES = [
  { days: 7, label: "overview.range7" },
  { days: 30, label: "overview.range30" },
  { days: 90, label: "overview.range90" },
] satisfies { days: number; label: TranslationKey }[];

const METRICS = [
  { key: "requests", label: "overview.metricRequests" },
  { key: "tokens", label: "overview.metricTokens" },
  { key: "cost_usd", label: "overview.metricCost" },
] satisfies { key: "requests" | "tokens" | "cost_usd"; label: TranslationKey }[];

type MetricKey = (typeof METRICS)[number]["key"];

export function OverviewPage() {
  const t = useT();
  const [days, setDays] = useState(7);
  const [metric, setMetric] = useState<MetricKey>("requests");

  const analytics = useResource(() => api.analyticsOverview(days), [days]);
  const providers = useResource(() => api.listProviders());
  const logs = useResource(() => api.listLogs({ limit: 6 }));

  const data = analytics.data;
  const series = data?.series ?? [];
  const totals = data?.totals;
  const previous = data?.previous;
  const enabledProviders = (providers.data ?? []).filter((p) => p.enabled !== false).length;
  const errorRate = totals?.requests ? (totals.errors / totals.requests) * 100 : 0;

  const chartConfig = {
    requests: { label: t("overview.metricRequests"), color: "var(--chart-1)" },
    tokens: { label: t("overview.metricTokens"), color: "var(--chart-2)" },
    cost_usd: { label: t("overview.metricCost"), color: "var(--chart-3)" },
  } satisfies ChartConfig;

  return (
    <>
      <PageHeader
        eyebrow={t("overview.eyebrow")}
        title={t("overview.title")}
        description={t("overview.description")}
        action={
          <SegmentedGroup label={t("overview.title")}>
            {RANGES.map((range) => (
              <SegmentedItem
                key={range.days}
                checked={days === range.days}
                onSelect={() => setDays(range.days)}
                label={t(range.label)}
                className="px-2.5"
              >
                {t(range.label)}
              </SegmentedItem>
            ))}
          </SegmentedGroup>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label={t("overview.requestsToday")}
          icon={ActivityIcon}
          value={formatNumber(totals?.requests ?? 0)}
          current={totals?.requests ?? 0}
          previous={previous?.requests ?? 0}
          windowDays={days}
          spark={normalise(series.map((b) => b.requests))}
        />
        <StatCard
          label={t("overview.tokensToday")}
          icon={CoinsIcon}
          value={formatNumber(totals?.tokens ?? 0)}
          current={totals?.tokens ?? 0}
          previous={previous?.tokens ?? 0}
          windowDays={days}
          spark={normalise(series.map((b) => b.tokens))}
        />
        <StatCard
          label={t("overview.spendToday")}
          icon={CoinsIcon}
          value={formatUSD(totals?.cost_usd ?? 0)}
          current={totals?.cost_usd ?? 0}
          previous={previous?.cost_usd ?? 0}
          windowDays={days}
          goodDirection="neutral"
          spark={normalise(series.map((b) => b.cost_usd))}
        />
        <StatCard
          label={t("overview.errors")}
          icon={TriangleAlertIcon}
          value={formatNumber(totals?.errors ?? 0)}
          current={totals?.errors ?? 0}
          previous={previous?.errors ?? 0}
          windowDays={days}
          goodDirection="down"
          footer={t("overview.errorRate", { rate: errorRate.toFixed(1) })}
          spark={normalise(series.map((b) => b.errors))}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>{t("overview.trendTitle")}</CardTitle>
            <CardDescription>{t("overview.trendDescription")}</CardDescription>
            <div className="col-start-2 row-span-2 row-start-1 self-start justify-self-end">
              <SegmentedGroup label={t("overview.trendTitle")}>
                {METRICS.map((m) => (
                  <SegmentedItem
                    key={m.key}
                    checked={metric === m.key}
                    onSelect={() => setMetric(m.key)}
                    label={t(m.label)}
                    className="px-2.5"
                  >
                    {t(m.label)}
                  </SegmentedItem>
                ))}
              </SegmentedGroup>
            </div>
          </CardHeader>
          <CardContent>
            <DataState loading={analytics.loading} error={analytics.error} rows={3}>
              <ChartContainer config={chartConfig} className="h-[260px] w-full">
                <AreaChart data={series} margin={{ left: 4, right: 8, top: 8 }}>
                  <defs>
                    <linearGradient id="metricFill" x1="0" y1="0" x2="0" y2="1">
                      <stop
                        offset="0%"
                        stopColor={`var(--color-${metric})`}
                        stopOpacity={0.35}
                      />
                      <stop
                        offset="100%"
                        stopColor={`var(--color-${metric})`}
                        stopOpacity={0.02}
                      />
                    </linearGradient>
                  </defs>
                  <CartesianGrid vertical={false} strokeDasharray="3 3" />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickMargin={8}
                    minTickGap={24}
                    tickFormatter={(value: string) => value.slice(5)}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    width={44}
                    tickFormatter={(value: number) =>
                      metric === "cost_usd" ? formatUSD(value) : compact(value)
                    }
                  />
                  <ChartTooltip content={<ChartTooltipContent indicator="line" />} />
                  <Area
                    dataKey={metric}
                    type="monotone"
                    stroke={`var(--color-${metric})`}
                    strokeWidth={2}
                    fill="url(#metricFill)"
                  />
                </AreaChart>
              </ChartContainer>
            </DataState>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("overview.shareTitle")}</CardTitle>
            <CardDescription>{t("overview.shareDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <DataState
              loading={analytics.loading}
              error={analytics.error}
              empty={(data?.by_provider ?? []).length === 0}
              emptyMessage={t("overview.noTraffic")}
              rows={3}
            >
              <ShareList rows={data?.by_provider ?? []} marks />
            </DataState>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>{t("overview.topModelsTitle")}</CardTitle>
            <CardDescription>{t("overview.topModelsDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <DataState
              loading={analytics.loading}
              error={analytics.error}
              empty={(data?.by_model ?? []).length === 0}
              emptyMessage={t("overview.noTraffic")}
              rows={3}
            >
              <ShareList rows={data?.by_model ?? []} />
            </DataState>
          </CardContent>
        </Card>

        <Card className="lg:col-span-2">
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
                      <TableCell className="text-muted-foreground">
                        {log.provider || "—"}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {log.latency_ms} ms
                      </TableCell>
                      <TableCell className="text-right">
                        <Badge
                          variant="secondary"
                          className={
                            log.status_code >= 400
                              ? "bg-destructive-subtle text-destructive-emphasis"
                              : "bg-success-subtle text-success-emphasis"
                          }
                        >
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
      </div>

      <div className="flex flex-wrap gap-2">
        <Button asChild variant="outline">
          <NavLink to="/providers">
            <ServerIcon />
            {t("overview.manageProviders")}
          </NavLink>
        </Button>
        <Button asChild variant="outline">
          <NavLink to="/logs">{t("overview.browseLogs")}</NavLink>
        </Button>
        <span className="text-muted-foreground self-center text-xs">
          {t("overview.configured", { count: formatNumber(enabledProviders) })}
        </span>
      </div>
    </>
  );
}

/**
 * A ranked list where the bar is the primary encoding and the number is
 * confirmation — easier to scan than a donut when the labels are long
 * model identifiers.
 */
function ShareList({ rows, marks = false }: { rows: AnalyticsBreakdown[]; marks?: boolean }) {
  const t = useT();
  const total = rows.reduce((sum, row) => sum + row.requests, 0) || 1;

  return (
    <div className="space-y-3">
      {rows.map((row, index) => {
        const percent = (row.requests / total) * 100;
        return (
          <div key={row.name} className="space-y-1.5">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="flex min-w-0 items-center gap-2">
                {marks ? (
                  <ProviderLogo name={row.name} className="size-5 rounded-[5px]" />
                ) : null}
                <span className="truncate font-medium">{row.name}</span>
              </span>
              <span className="text-muted-foreground shrink-0 tabular-nums">
                {formatNumber(row.requests)}
              </span>
            </div>
            <div className="bg-quota-track h-1.5 overflow-hidden rounded-full">
              <div
                className="h-full rounded-full"
                style={{
                  width: `${Math.max(percent, 2)}%`,
                  backgroundColor: `var(--chart-${(index % 5) + 1})`,
                }}
              />
            </div>
            <p className="text-muted-foreground text-xs">
              {t("overview.ofRequests", { percent: percent.toFixed(0) })}
            </p>
          </div>
        );
      })}
    </div>
  );
}

/** 12.4k rather than 12,400 — axis labels have no room for separators. */
function compact(value: number) {
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}
