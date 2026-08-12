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
  const usage = useResource(() => api.usageSummary());
  const providers = useResource(() => api.listProviders());
  const logs = useResource(() => api.listLogs({ limit: 8 }));

  const daily = usage.data?.daily;
  const monthly = usage.data?.monthly;
  const enabledProviders = (providers.data ?? []).filter((p) => p.enabled !== false).length;

  return (
    <>
      <PageHeader
        title="Overview"
        description="Traffic, spend and provider health across the gateway."
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="Requests today"
          value={formatNumber(daily?.Requests ?? 0)}
          hint={`${formatNumber(monthly?.Requests ?? 0)} this month`}
        />
        <StatCard
          label="Tokens today"
          value={formatNumber(daily?.Tokens ?? 0)}
          hint={`${formatNumber(monthly?.Tokens ?? 0)} this month`}
        />
        <StatCard
          label="Spend today"
          value={formatUSD(daily?.CostUSD ?? 0)}
          hint={`${formatUSD(monthly?.CostUSD ?? 0)} this month`}
        />
        <StatCard
          label="Providers"
          value={formatNumber(enabledProviders)}
          hint={`${formatNumber(providers.data?.length ?? 0)} configured`}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Recent requests</CardTitle>
          <CardDescription>The last calls to reach the data plane.</CardDescription>
        </CardHeader>
        <CardContent>
          <DataState
            loading={logs.loading}
            error={logs.error}
            empty={(logs.data?.data ?? []).length === 0}
            emptyMessage="No requests recorded yet. Point a client at /v1 to see traffic here."
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>When</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead className="text-right">Tokens</TableHead>
                  <TableHead className="text-right">Latency</TableHead>
                  <TableHead className="text-right">Status</TableHead>
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
          <CardTitle>Providers</CardTitle>
          <CardDescription>Upstreams the router can currently reach for.</CardDescription>
        </CardHeader>
        <CardContent>
          <DataState
            loading={providers.loading}
            error={providers.error}
            empty={(providers.data ?? []).length === 0}
            emptyMessage="No providers configured yet."
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
          <NavLink to="/providers">Manage providers</NavLink>
        </Button>
        <Button asChild variant="outline">
          <NavLink to="/logs">Browse request logs</NavLink>
        </Button>
      </div>
    </>
  );
}
