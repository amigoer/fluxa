import { useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Copy,
  Inbox,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { SegmentedControl } from "@/components/ui/segmented-control";
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
import { useT, type TranslationKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";
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
  const [initialLoaded, setInitialLoaded] = useState(false);

  // Filter inputs are kept in local state and only applied on submit
  // (Enter / segmented onChange) so typing in the search box does not
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
      setInitialLoaded(true);
    }
  };

  useEffect(() => {
    loadRef.current();
  }, [committed, status, stream, offset]);

  function commitText() {
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

  const pageFrom = total === 0 ? 0 : offset + 1;
  const pageTo = Math.min(offset + PAGE_SIZE, total);
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

      {/* Filter bar — two rows. The top row is the always-visible search
          + global actions. The bottom row is the detail filters which
          auto-apply (segmented controls) or apply on Enter (text). */}
      <Card>
        <CardContent className="p-4 space-y-3">
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && commitText()}
                placeholder={t("logs.searchPlaceholder")}
                className="h-10 pl-9 pr-24"
              />
              <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-muted-foreground/70">
                {t("logs.searchHint")}
              </span>
            </div>
            {filtersDirty && (
              <Button variant="ghost" size="sm" onClick={clearFilters}>
                <X className="h-3.5 w-3.5" />
                {t("logs.clear")}
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => loadRef.current()}
              disabled={loading}
            >
              <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
              {t("logs.refresh")}
            </Button>
          </div>

          <div className="flex flex-wrap items-end gap-x-4 gap-y-3 pt-1">
            <FilterField label={t("logs.filterKey")} className="min-w-[180px] flex-1">
              <Input
                value={keyInput}
                onChange={(e) => setKeyInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && commitText()}
                placeholder="vk-…"
                className="h-9"
              />
            </FilterField>
            <FilterField label={t("logs.filterModel")} className="min-w-[180px] flex-1">
              <Input
                value={modelInput}
                onChange={(e) => setModelInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && commitText()}
                placeholder="gpt-4o"
                className="h-9"
              />
            </FilterField>
            <FilterField label={t("logs.filterStatus")}>
              <SegmentedControl<StatusBucket>
                value={status}
                onChange={(v) => {
                  setOffset(0);
                  setStatus(v);
                }}
                size="sm"
                options={[
                  { label: t("logs.filterStatusAll"), value: "all" },
                  { label: t("logs.filterStatus2xx"), value: "2xx" },
                  { label: t("logs.filterStatus4xx"), value: "4xx" },
                  { label: t("logs.filterStatus5xx"), value: "5xx" },
                ]}
              />
            </FilterField>
            <FilterField label={t("logs.filterStream")}>
              <SegmentedControl<StreamBucket>
                value={stream}
                onChange={(v) => {
                  setOffset(0);
                  setStream(v);
                }}
                size="sm"
                options={[
                  { label: t("logs.filterStreamAll"), value: "all" },
                  { label: t("logs.filterStreamOn"), value: "stream" },
                  { label: t("logs.filterStreamOff"), value: "nostream" },
                ]}
              />
            </FilterField>
          </div>
        </CardContent>
      </Card>

      {/* Log table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
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
              {!initialLoaded && loading && (
                <SkeletonRows count={8} />
              )}
              {initialLoaded &&
                rows.map((row) => (
                  <TableRow
                    key={row.id}
                    className="cursor-pointer transition-colors"
                    onClick={() => openRow(row.id)}
                  >
                    <TableCell
                      className="text-xs text-muted-foreground"
                      title={formatTime(row.started_at)}
                    >
                      {formatRelativeTime(t, row.started_at)}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {row.virtual_key_id ? (
                        <span title={row.virtual_key_id}>
                          {row.virtual_key_id.slice(0, 14)}…
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <span>{row.model_requested || row.model_resolved}</span>
                        {row.is_stream && <StreamBadge />}
                      </div>
                      {row.model_resolved &&
                        row.model_resolved !== row.model_requested && (
                          <div className="text-[11px] text-muted-foreground font-mono">
                            → {row.model_resolved}
                          </div>
                        )}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {row.provider || "—"}
                    </TableCell>
                    <TableCell>
                      <StatusPill status={row.status_code} />
                    </TableCell>
                    <TableCell className="text-right font-mono text-xs">
                      <span className="text-muted-foreground">
                        {formatTokens(row.prompt_tokens)} /{" "}
                        {formatTokens(row.completion_tokens)} /{" "}
                      </span>
                      <span className="font-medium text-foreground">
                        {formatTokens(row.total_tokens)}
                      </span>
                    </TableCell>
                    <TableCell className="text-right font-mono text-xs">
                      {formatLatency(row.latency_ms)}
                    </TableCell>
                    <TableCell className="text-right font-mono text-xs text-muted-foreground">
                      {row.ttft_ms > 0 ? formatLatency(row.ttft_ms) : "—"}
                    </TableCell>
                    <TableCell className="text-right font-mono text-xs">
                      {formatCost(row.cost_usd)}
                    </TableCell>
                  </TableRow>
                ))}
              {initialLoaded && rows.length === 0 && (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={9} className="py-16">
                    <EmptyState
                      filtered={filtersDirty}
                      onClear={clearFilters}
                    />
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
            {t("logs.pageOf", { from: pageFrom, to: pageTo, total })}
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

      <Dialog
        open={activeLog !== null}
        onOpenChange={(open) => !open && setActiveLog(null)}
      >
        <DialogContent className="!flex max-h-[90vh] !max-w-3xl !flex-col overflow-hidden">
          {activeLog && <LogDetailPanel detail={activeLog} loading={activeLoading} />}
        </DialogContent>
      </Dialog>
    </div>
  );
}

// FilterField wraps a label and its control with consistent spacing so
// rows of mixed inputs (text + segmented) line up at the baseline.
function FilterField({
  label,
  children,
  className,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <Label className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">
        {label}
      </Label>
      {children}
    </div>
  );
}

// SkeletonRows paints loading placeholders that mirror the live row
// shape, so the table does not collapse to a single empty cell while
// the first request is in flight.
function SkeletonRows({ count }: { count: number }) {
  return (
    <>
      {Array.from({ length: count }).map((_, i) => (
        <TableRow key={`skeleton-${i}`} className="hover:bg-transparent">
          {Array.from({ length: 9 }).map((__, j) => (
            <TableCell key={j}>
              <Skeleton className="h-4 w-full" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  );
}

function EmptyState({
  filtered,
  onClear,
}: {
  filtered: boolean;
  onClear: () => void;
}) {
  const { t } = useT();
  return (
    <div className="flex flex-col items-center gap-3 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Inbox className="h-5 w-5" />
      </div>
      <div className="space-y-1">
        <p className="text-sm font-medium text-foreground">
          {filtered ? t("logs.emptySearch") : t("logs.empty")}
        </p>
        {!filtered && (
          <p className="max-w-sm text-xs text-muted-foreground">
            {t("logs.emptyHint")}
          </p>
        )}
      </div>
      {filtered && (
        <Button variant="outline" size="sm" onClick={onClear}>
          <X className="h-3.5 w-3.5" />
          {t("logs.clear")}
        </Button>
      )}
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

// StreamBadge marks a row as a streaming response. Kept tiny so the
// model column stays scannable; the colour is the same blue we use
// in the route graph for stream edges so the visual vocabulary is
// consistent.
function StreamBadge() {
  const { t } = useT();
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-600 dark:text-blue-400">
      <Activity className="h-2.5 w-2.5" />
      {t("logs.streamBadge")}
    </span>
  );
}

// StatusPill colours the status_code by class so the rare 4xx/5xx
// pops without making the common 200 row visually noisy.
function StatusPill({ status }: { status: number }) {
  let tone = "bg-muted text-muted-foreground";
  if (status >= 200 && status < 300)
    tone = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
  else if (status >= 400 && status < 500)
    tone = "bg-amber-500/10 text-amber-600 dark:text-amber-400";
  else if (status >= 500)
    tone = "bg-rose-500/10 text-rose-600 dark:text-rose-400";
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold",
        tone,
      )}
    >
      {status || "—"}
    </span>
  );
}

// formatTime returns a locale-aware absolute timestamp. Used as the
// tooltip on relative-time cells so hovering still gives the exact
// value when "5m ago" is not precise enough.
function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

// formatRelativeTime returns "just now / Xm ago / Xh ago / Xd ago"
// for recent timestamps and falls back to an absolute date when the
// value is older than a week. i18n is threaded through `t` so the
// Chinese locale renders the same buckets natively.
function formatRelativeTime(
  t: (key: TranslationKey, params?: Record<string, string | number>) => string,
  iso: string,
): string {
  try {
    const ms = Date.now() - new Date(iso).getTime();
    if (ms < 60_000) return t("logs.timeJustNow");
    if (ms < 3_600_000) return t("logs.timeMinutesAgo", { n: Math.floor(ms / 60_000) });
    if (ms < 86_400_000) return t("logs.timeHoursAgo", { n: Math.floor(ms / 3_600_000) });
    if (ms < 7 * 86_400_000) return t("logs.timeDaysAgo", { n: Math.floor(ms / 86_400_000) });
    return new Date(iso).toLocaleDateString();
  } catch {
    return iso;
  }
}

// formatLatency picks ms or s based on magnitude. Anything ≥ 1s
// reads better as seconds with two decimals than as a four-digit
// millisecond count.
function formatLatency(ms: number): string {
  if (!ms) return "—";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

// formatCost picks precision based on magnitude. Tiny costs (sub-
// cent) get more decimals so a 0.0001 / 0.000001 difference does
// not collapse to "$0.00"; dollar-level costs get the usual two.
function formatCost(usd: number): string {
  if (!usd) return "—";
  if (usd >= 1) return `$${usd.toFixed(2)}`;
  if (usd >= 0.01) return `$${usd.toFixed(4)}`;
  if (usd >= 0.000001) return `$${usd.toFixed(6)}`;
  return `<$${(0.000001).toFixed(6)}`;
}

// formatTokens adds thousands separators so 12 345 reads as easily
// as 12 — admins comparing per-request token counts across rows
// benefit from the visual delimiter.
function formatTokens(n: number): string {
  return n.toLocaleString();
}

// formatBytes converts a byte count to a human label. Used to show
// payload size next to body section titles in the detail dialog so
// the operator knows up front whether the body is small or capped.
function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
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
    {
      label: t("logs.detailEndpoint"),
      value: <code className="font-mono">{detail.endpoint}</code>,
    },
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
      value: detail.is_stream
        ? t("logs.filterStreamOn")
        : t("logs.filterStreamOff"),
    },
    {
      label: t("logs.colStatus"),
      value: <StatusPill status={detail.status_code} />,
    },
    {
      label: t("logs.detailTokens"),
      value: (
        <span className="font-mono">
          {formatTokens(detail.prompt_tokens)} /{" "}
          {formatTokens(detail.completion_tokens)} /{" "}
          {formatTokens(detail.total_tokens)}
        </span>
      ),
    },
    {
      label: t("logs.detailLatencyMs"),
      value: <span className="font-mono">{formatLatency(detail.latency_ms)}</span>,
    },
    {
      label: t("logs.detailTTFTMs"),
      value: (
        <span className="font-mono">
          {detail.ttft_ms > 0 ? formatLatency(detail.ttft_ms) : "—"}
        </span>
      ),
    },
    {
      label: t("logs.detailCost"),
      value: <span className="font-mono">{formatCost(detail.cost_usd)}</span>,
    },
    { label: t("logs.detailStartedAt"), value: formatTime(detail.started_at) },
    {
      label: t("logs.detailFirstByteAt"),
      value: detail.first_byte_at ? formatTime(detail.first_byte_at) : "—",
    },
    {
      label: t("logs.detailCompletedAt"),
      value: formatTime(detail.completed_at),
    },
    { label: t("logs.detailClientIP"), value: detail.client_ip || "—" },
    {
      label: t("logs.detailUserAgent"),
      value: (
        <span className="truncate" title={detail.user_agent}>
          {detail.user_agent || "—"}
        </span>
      ),
    },
  ];

  return (
    <>
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          {t("logs.detailTitle")}
          <StatusPill status={detail.status_code} />
          {detail.is_stream && <StreamBadge />}
        </DialogTitle>
        <DialogDescription className="font-mono text-xs">
          {detail.id}
        </DialogDescription>
      </DialogHeader>

      <div className="-mx-6 flex-1 space-y-5 overflow-y-auto px-6">
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
          <div className="grid grid-cols-1 gap-x-6 gap-y-1.5 text-sm md:grid-cols-2">
            {meta.map((m) => (
              <div
                key={m.label}
                className="flex items-baseline justify-between gap-3 border-b border-border/40 py-1"
              >
                <span className="text-xs uppercase tracking-wider text-muted-foreground">
                  {m.label}
                </span>
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
  const byteLength = useMemo(
    () => new TextEncoder().encode(body).length,
    [body],
  );

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
        <h3 className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
          {body && (
            <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-mono normal-case tracking-normal text-muted-foreground">
              {formatBytes(byteLength)}
            </span>
          )}
        </h3>
        {body && (
          <Button variant="ghost" size="sm" onClick={copy}>
            <Copy className="h-3.5 w-3.5" />
            {copied ? t("logs.detailCopied") : t("logs.detailCopy")}
          </Button>
        )}
      </div>
      {body ? (
        <pre className="max-h-96 overflow-x-auto whitespace-pre-wrap break-all rounded-md border border-border bg-muted/40 p-3 font-mono text-[12px] leading-relaxed">
          {pretty}
        </pre>
      ) : (
        <div className="text-xs italic text-muted-foreground">
          {t("logs.detailNoBody")}
        </div>
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
