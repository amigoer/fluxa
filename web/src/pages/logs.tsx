import { useState } from "react";
import { RotateCwIcon } from "lucide-react";

import { DataState, PageHeader } from "@/components/page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useResource } from "@/hooks/use-resource";
import { api, type RequestLog, type RequestLogFilter } from "@/lib/api";
import { formatDateTime, formatNumber, formatUSD, prettyJSON } from "@/lib/format";

const PAGE_SIZE = 25;

// The status filter maps to the API's status_min/status_max pair rather
// than an enum, so keep the translation in one table.
const STATUS_FILTERS = {
  all: {},
  success: { status_min: 200, status_max: 299 },
  client: { status_min: 400, status_max: 499 },
  server: { status_min: 500, status_max: 599 },
} satisfies Record<string, Partial<RequestLogFilter>>;

type StatusFilter = keyof typeof STATUS_FILTERS;

export function LogsPage() {
  const [search, setSearch] = useState("");
  const [applied, setApplied] = useState("");
  const [model, setModel] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [stream, setStream] = useState<"all" | "true" | "false">("all");
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);

  const filter: RequestLogFilter = {
    ...STATUS_FILTERS[status],
    search: applied || undefined,
    model: model || undefined,
    stream: stream === "all" ? undefined : stream === "true",
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  };

  const logs = useResource(
    () => api.listLogs(filter),
    [applied, model, status, stream, page],
  );

  const total = logs.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Any filter change invalidates the current page number.
  const resetAnd = <T,>(setter: (value: T) => void) => (value: T) => {
    setPage(0);
    setter(value);
  };

  return (
    <>
      <PageHeader
        title="Request Logs"
        description="Every call through /v1, with the payloads the upstream actually saw."
        action={
          <Button variant="outline" onClick={logs.reload}>
            <RotateCwIcon />
            Refresh
          </Button>
        }
      />

      <Card>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <form
            className="space-y-2"
            onSubmit={(event) => {
              event.preventDefault();
              setPage(0);
              setApplied(search);
            }}
          >
            <Label htmlFor="log-search">Search bodies</Label>
            <Input
              id="log-search"
              placeholder="Press Enter to search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </form>

          <div className="space-y-2">
            <Label htmlFor="log-model">Model</Label>
            <Input
              id="log-model"
              placeholder="exact match"
              value={model}
              onChange={(event) => resetAnd(setModel)(event.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="log-status">Status</Label>
            <Select value={status} onValueChange={resetAnd((v) => setStatus(v as StatusFilter))}>
              <SelectTrigger id="log-status" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="success">2xx success</SelectItem>
                <SelectItem value="client">4xx client error</SelectItem>
                <SelectItem value="server">5xx server error</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="log-stream">Streaming</Label>
            <Select
              value={stream}
              onValueChange={resetAnd((v) => setStream(v as "all" | "true" | "false"))}
            >
              <SelectTrigger id="log-stream" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="true">Streaming only</SelectItem>
                <SelectItem value="false">Non-streaming only</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <DataState
        loading={logs.loading}
        error={logs.error}
        empty={(logs.data?.data ?? []).length === 0}
        emptyMessage="No requests match this filter."
        rows={6}
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Started</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead className="text-right">Tokens</TableHead>
                  <TableHead className="text-right">Cost</TableHead>
                  <TableHead className="text-right">Latency</TableHead>
                  <TableHead className="text-right">Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(logs.data?.data ?? []).map((log) => (
                  <TableRow
                    key={log.id}
                    onClick={() => setSelected(log.id)}
                    className="cursor-pointer"
                  >
                    <TableCell className="text-muted-foreground whitespace-nowrap">
                      {formatDateTime(log.started_at)}
                    </TableCell>
                    <TableCell className="font-medium">
                      {log.model_resolved || log.model_requested || "—"}
                      {log.is_stream ? (
                        <Badge variant="outline" className="ml-2">
                          stream
                        </Badge>
                      ) : null}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{log.provider || "—"}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(log.total_tokens)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatUSD(log.cost_usd)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{log.latency_ms} ms</TableCell>
                    <TableCell className="text-right">
                      <StatusBadge log={log} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </DataState>

      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-sm">
          {formatNumber(total)} requests · page {page + 1} of {pageCount}
        </p>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 0}
            onClick={() => setPage((p) => Math.max(0, p - 1))}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page + 1 >= pageCount}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      </div>

      <LogDetailSheet id={selected} onClose={() => setSelected(null)} />
    </>
  );
}

function StatusBadge({ log }: { log: RequestLog }) {
  if (log.status_code >= 500) return <Badge variant="destructive">{log.status_code}</Badge>;
  if (log.status_code >= 400) return <Badge variant="destructive">{log.status_code}</Badge>;
  return <Badge variant="secondary">{log.status_code}</Badge>;
}

function LogDetailSheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  // Only fetch once a row is picked; the id doubles as the cache key.
  const detail = useResource(
    () => (id ? api.getLog(id) : Promise.resolve(null)),
    [id],
  );

  const log = detail.data;

  return (
    <Sheet open={id !== null} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full gap-0 sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>Request detail</SheetTitle>
          <SheetDescription>
            {log ? `${log.method} ${log.endpoint}` : "Loading…"}
          </SheetDescription>
        </SheetHeader>

        {/* A plain scroll container rather than ScrollArea: Radix renders
            its viewport as display:table, which stops long ids and URLs
            from wrapping inside the fixed-width sheet. */}
        <div className="h-[calc(100svh-6rem)] overflow-y-auto">
          <div className="space-y-6 px-4 pb-8">
            <DataState loading={detail.loading} error={detail.error} rows={3}>
              {log ? (
                <>
                  <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                    <Field label="Status" value={String(log.status_code)} />
                    <Field label="Started" value={formatDateTime(log.started_at)} />
                    <Field label="Model requested" value={log.model_requested || "—"} />
                    <Field label="Model resolved" value={log.model_resolved || "—"} />
                    <Field label="Provider" value={log.provider || "—"} />
                    <Field label="Virtual key" value={log.virtual_key_id || "—"} />
                    <Field label="Tokens" value={formatNumber(log.total_tokens)} />
                    <Field label="Cost" value={formatUSD(log.cost_usd)} />
                    <Field label="Latency" value={`${log.latency_ms} ms`} />
                    <Field label="TTFT" value={log.ttft_ms ? `${log.ttft_ms} ms` : "—"} />
                    <Field label="Client IP" value={log.client_ip || "—"} />
                    <Field label="Stream" value={log.is_stream ? "yes" : "no"} />
                  </dl>

                  {log.error ? (
                    <div>
                      <h3 className="mb-2 text-sm font-medium">Error</h3>
                      <p className="text-destructive text-sm">{log.error}</p>
                    </div>
                  ) : null}

                  <Payload title="Request body" body={log.request_body} />
                  <Payload title="Response body" body={log.response_body} />
                </>
              ) : null}
            </DataState>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-muted-foreground text-xs">{label}</dt>
      <dd className="font-medium break-all">{value}</dd>
    </div>
  );
}

function Payload({ title, body }: { title: string; body: string }) {
  return (
    <div>
      <h3 className="mb-2 text-sm font-medium">{title}</h3>
      {body ? (
        <pre className="bg-muted max-h-80 overflow-auto rounded-md p-3 text-xs">
          {prettyJSON(body)}
        </pre>
      ) : (
        <p className="text-muted-foreground text-sm">
          Not captured. Set FLUXA_STORE_CONTENT=true to record payloads.
        </p>
      )}
    </div>
  );
}
