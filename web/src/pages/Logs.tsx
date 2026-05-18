import { useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Copy,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Logs, type RequestLogDetail, type RequestLogSummary } from "@/lib/api";
import { useT } from "@/lib/i18n";
import { toast } from "sonner";

// PAGE_SIZE keeps requests cheap and the table readable. The server
// caps the limit at 500 anyway; we sit comfortably below that so a
// page never feels overwhelming even at high traffic.
const PAGE_SIZE = 50;

type StatusBucket = "all" | "2xx" | "4xx" | "5xx";
type StreamBucket = "all" | "stream" | "nostream";

// LogsPage lists request_logs rows with filters, pagination, and a
// detail drawer that shows the full request/response bodies. Body
// fetching is deferred to row-click so the list view stays light.
export function LogsPage() {
  const { t } = useT();
  const [rows, setRows] = useState<RequestLogSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);

  // Filter inputs are kept in local state and only applied on submit
  // (Enter / button click) so typing in the search box does not
  // hammer the server. The committed values live alongside them.
  const [searchInput, setSearchInput] = useState("");
  const [keyInput, setKeyInput] = useState("");
  const [modelInput, setModelInput] = useState("");
  const [committed, setCommitted] = useState({ search: "", key: "", model: "" });
  const [status, setStatus] = useState<StatusBucket>("all");
  const [stream, setStream] = useState<StreamBucket>("all");

  const [activeLog, setActiveLog] = useState<RequestLogDetail | null>(null);
  const [activeLoading, setActiveLoading] = useState(false);

  // load() reads the currently committed filters + pagination state
  // and pulls one page from the server. Errors land as a toast so
  // the table itself stays visible (last successful page is kept).
  const loadRef = useRef<() => void>(() => {});
  loadRef.current = async function load() {
    setLoading(true);
    try {
      const filter = buildFilter({
        committed,
        status,
        stream,
        offset,
        limit: PAGE_SIZE,
      });
      const resp = await Logs.list(filter);
      setRows(resp.data ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRef.current();
  }, [committed, status, stream, offset]);

  function applyFilters() {
    setOffset(0);
    setCommitted({ search: searchInput, key: keyInput, model: modelInput });
  }

  function clearFilters() {
    setSearchInput("");
    setKeyInput("");
    setModelInput("");
    setStatus("all");
    setStream("all");
    setOffset(0);
    setCommitted({ search: "", key: "", model: "" });
  }

  async function openRow(id: string) {
    setActiveLoading(true);
    try {
      const detail = await Logs.get(id);
      setActiveLog(detail);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setActiveLoading(false);
    }
  }

  const pageInfo = useMemo(() => {
    if (total === 0) return { from: 0, to: 0 };
    return {
      from: offset + 1,
      to: Math.min(offset + PAGE_SIZE, total),
    };
  }, [offset, total]);

  const hasNext = offset + PAGE_SIZE < total;
  const hasPrev = offset > 0;
  const filtersDirty =
    committed.search !== "" ||
    committed.key !== "" ||
    committed.model !== "" ||
    status !== "all" ||
    stream !== "all";

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("logs.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("logs.subtitle")}</p>
      </div>

      {/* Filter bar */}
      <Card>
        <CardContent className="p-4 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="space-y-1.5 md:col-span-3">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {t("logs.searchPlaceholder")}
              </Label>
              <div className="relative">
                <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && applyFilters()}
                  placeholder={t("logs.searchPlaceholder")}
                  className="pl-9"
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {t("logs.filterKey")}
              </Label>
              <Input
                value={keyInput}
                onChange={(e) => setKeyInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && applyFilters()}
                placeholder="vk-…"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {t("logs.filterModel")}
              </Label>
              <Input
                value={modelInput}
                onChange={(e) => setModelInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && applyFilters()}
                placeholder="gpt-4o"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {t("logs.filterStatus")}
              </Label>
              <select
                value={status}
                onChange={(e) => {
                  setOffset(0);
                  setStatus(e.target.value as StatusBucket);
                }}
                className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="all">{t("logs.filterStatusAll")}</option>
                <option value="2xx">{t("logs.filterStatus2xx")}</option>
                <option value="4xx">{t("logs.filterStatus4xx")}</option>
                <option value="5xx">{t("logs.filterStatus5xx")}</option>
              </select>
            </div>
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {t("logs.filterStream")}
              </Label>
              <select
                value={stream}
                onChange={(e) => {
                  setOffset(0);
                  setStream(e.target.value as StreamBucket);
                }}
                className="h-9 rounded-md border border-input bg-transparent px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="all">{t("logs.filterStreamAll")}</option>
                <option value="stream">{t("logs.filterStreamOn")}</option>
                <option value="nostream">{t("logs.filterStreamOff")}</option>
              </select>
            </div>
            <div className="flex items-end gap-2 ml-auto">
              {filtersDirty && (
                <Button variant="ghost" size="sm" onClick={clearFilters}>
                  <X className="h-3.5 w-3.5" /> {t("logs.clear")}
                </Button>
              )}
              <Button onClick={applyFilters} size="sm">
                <Search className="h-3.5 w-3.5" /> {t("logs.searchPlaceholder").split("…")[0]}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => loadRef.current()}
                disabled={loading}
              >
                <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
                {t("logs.refresh")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Log table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[160px]">{t("logs.colTime")}</TableHead>
                <TableHead>{t("logs.colKey")}</TableHead>
                <TableHead>{t("logs.colModel")}</TableHead>
                <TableHead>{t("logs.colProvider")}</TableHead>
                <TableHead className="w-[88px]">{t("logs.colStatus")}</TableHead>
                <TableHead className="text-right">{t("logs.colTokens")}</TableHead>
                <TableHead className="text-right w-[96px]">{t("logs.colLatency")}</TableHead>
                <TableHead className="text-right w-[80px]">{t("logs.colTTFT")}</TableHead>
                <TableHead className="text-right w-[96px]">{t("logs.colCost")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="cursor-pointer"
                  onClick={() => openRow(row.id)}
                >
                  <TableCell className="text-xs text-muted-foreground">
                    {formatTime(row.started_at)}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {row.virtual_key_id ? row.virtual_key_id.slice(0, 14) + "…" : "—"}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <span>{row.model_requested || row.model_resolved}</span>
                      {row.is_stream && (
                        <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-600 dark:text-blue-400">
                          <Activity className="h-2.5 w-2.5" />
                          {t("logs.streamBadge")}
                        </span>
                      )}
                    </div>
                    {row.model_resolved &&
                      row.model_resolved !== row.model_requested && (
                        <div className="text-[11px] text-muted-foreground font-mono">
                          → {row.model_resolved}
                        </div>
                      )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">{row.provider || "—"}</TableCell>
                  <TableCell>
                    <StatusPill status={row.status_code} />
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {row.prompt_tokens} / {row.completion_tokens} /{" "}
                    <span className="font-medium">{row.total_tokens}</span>
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {row.latency_ms}ms
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs text-muted-foreground">
                    {row.ttft_ms > 0 ? `${row.ttft_ms}ms` : "—"}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {row.cost_usd > 0 ? `$${row.cost_usd.toFixed(5)}` : "—"}
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={9} className="text-center text-muted-foreground py-12">
                    {filtersDirty ? t("logs.emptySearch") : t("logs.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            {t("logs.pageOf", {
              from: pageInfo.from,
              to: pageInfo.to,
              total,
            })}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={!hasPrev || loading}
            >
              <ChevronLeft className="h-3.5 w-3.5" />
              {t("logs.previousPage")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={!hasNext || loading}
            >
              {t("logs.nextPage")}
              <ChevronRight className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      )}

      <Dialog open={activeLog !== null} onOpenChange={(open) => !open && setActiveLog(null)}>
        <DialogContent className="!flex max-h-[90vh] !max-w-3xl !flex-col overflow-hidden">
          {activeLog && (
            <LogDetailPanel detail={activeLog} loading={activeLoading} />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function buildFilter({
  committed,
  status,
  stream,
  offset,
  limit,
}: {
  committed: { search: string; key: string; model: string };
  status: StatusBucket;
  stream: StreamBucket;
  offset: number;
  limit: number;
}) {
  const filter: {
    search?: string;
    key_id?: string;
    model?: string;
    status_min?: number;
    status_max?: number;
    stream?: boolean;
    offset: number;
    limit: number;
  } = { offset, limit };
  if (committed.search) filter.search = committed.search;
  if (committed.key) filter.key_id = committed.key;
  if (committed.model) filter.model = committed.model;
  if (status === "2xx") {
    filter.status_min = 200;
    filter.status_max = 299;
  } else if (status === "4xx") {
    filter.status_min = 400;
    filter.status_max = 499;
  } else if (status === "5xx") {
    filter.status_min = 500;
    filter.status_max = 599;
  }
  if (stream === "stream") filter.stream = true;
  if (stream === "nostream") filter.stream = false;
  return filter;
}

// StatusPill colours the status_code by class to keep the eye on the
// rare 4xx/5xx without making the common 200 row visually noisy.
function StatusPill({ status }: { status: number }) {
  let tone = "bg-muted text-muted-foreground";
  if (status >= 200 && status < 300) tone = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
  else if (status >= 400 && status < 500) tone = "bg-amber-500/10 text-amber-600 dark:text-amber-400";
  else if (status >= 500) tone = "bg-rose-500/10 text-rose-600 dark:text-rose-400";
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${tone}`}>
      {status || "—"}
    </span>
  );
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

// LogDetailPanel renders the full request_log row inside the dialog,
// split into metadata grid + two body panels with copy-to-clipboard
// buttons. Both bodies are shown verbatim — even valid JSON keeps the
// original whitespace so the operator sees exactly what crossed the
// wire.
function LogDetailPanel({
  detail,
  loading,
}: {
  detail: RequestLogDetail;
  loading: boolean;
}) {
  const { t } = useT();

  const meta: Array<{ label: string; value: React.ReactNode }> = [
    { label: t("logs.detailEndpoint"), value: <code className="font-mono">{detail.endpoint}</code> },
    { label: t("logs.detailMethod"), value: detail.method },
    {
      label: t("logs.detailModelRequested"),
      value: <code className="font-mono">{detail.model_requested}</code>,
    },
    {
      label: t("logs.detailModelResolved"),
      value: <code className="font-mono">{detail.model_resolved}</code>,
    },
    { label: t("logs.detailProvider"), value: detail.provider || "—" },
    {
      label: t("logs.detailIsStream"),
      value: detail.is_stream ? t("logs.filterStreamOn") : t("logs.filterStreamOff"),
    },
    { label: t("logs.colStatus"), value: <StatusPill status={detail.status_code} /> },
    {
      label: t("logs.detailTokens"),
      value: (
        <span className="font-mono">
          {detail.prompt_tokens} / {detail.completion_tokens} / {detail.total_tokens}
        </span>
      ),
    },
    {
      label: t("logs.detailLatencyMs"),
      value: <span className="font-mono">{detail.latency_ms}ms</span>,
    },
    {
      label: t("logs.detailTTFTMs"),
      value: (
        <span className="font-mono">
          {detail.ttft_ms > 0 ? `${detail.ttft_ms}ms` : "—"}
        </span>
      ),
    },
    {
      label: t("logs.detailCost"),
      value: (
        <span className="font-mono">
          {detail.cost_usd > 0 ? `$${detail.cost_usd.toFixed(6)}` : "—"}
        </span>
      ),
    },
    { label: t("logs.detailStartedAt"), value: formatTime(detail.started_at) },
    {
      label: t("logs.detailFirstByteAt"),
      value: detail.first_byte_at ? formatTime(detail.first_byte_at) : "—",
    },
    { label: t("logs.detailCompletedAt"), value: formatTime(detail.completed_at) },
    { label: t("logs.detailClientIP"), value: detail.client_ip || "—" },
    { label: t("logs.detailUserAgent"), value: detail.user_agent || "—" },
  ];

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("logs.detailTitle")}</DialogTitle>
        <DialogDescription className="font-mono text-xs">{detail.id}</DialogDescription>
      </DialogHeader>

      <div className="flex-1 overflow-y-auto -mx-6 px-6 space-y-5">
        {loading && <div className="text-sm text-muted-foreground">Loading…</div>}

        {detail.error && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t("logs.detailError")}
            </h3>
            <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {detail.error}
            </div>
          </section>
        )}

        <section className="space-y-2">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {t("logs.detailMetadata")}
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1.5 text-sm">
            {meta.map((m) => (
              <div key={m.label} className="flex items-baseline justify-between gap-3 border-b border-border/40 py-1">
                <span className="text-muted-foreground text-xs uppercase tracking-wider">{m.label}</span>
                <span className="text-right">{m.value}</span>
              </div>
            ))}
          </div>
        </section>

        <BodySection title={t("logs.detailRequest")} body={detail.request_body} />
        <BodySection title={t("logs.detailResponse")} body={detail.response_body} />
      </div>
    </>
  );
}

function BodySection({ title, body }: { title: string; body: string }) {
  const { t } = useT();
  const [copied, setCopied] = useState(false);
  const pretty = useMemo(() => prettyJSON(body), [body]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(body);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("clipboard write failed");
    }
  }

  return (
    <section className="space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </h3>
        {body && (
          <Button variant="ghost" size="sm" onClick={copy}>
            <Copy className="h-3.5 w-3.5" />
            {copied ? t("logs.detailCopied") : t("logs.detailCopy")}
          </Button>
        )}
      </div>
      {body ? (
        <pre className="rounded-md border border-border bg-muted/40 p-3 text-[12px] leading-relaxed font-mono overflow-x-auto whitespace-pre-wrap break-all max-h-96">
          {pretty}
        </pre>
      ) : (
        <div className="text-xs text-muted-foreground italic">{t("logs.detailNoBody")}</div>
      )}
    </section>
  );
}

// prettyJSON tries to JSON-format a body if it parses cleanly; if not
// (SSE-framed streams, malformed payloads) it returns the input as-is
// so the operator still sees what the wire actually carried.
function prettyJSON(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}
