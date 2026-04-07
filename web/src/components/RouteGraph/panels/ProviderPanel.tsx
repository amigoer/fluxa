// ProviderPanel — read-only summary of a (provider, model) tuple. The
// graph is a routing view, not a provider editor; for full provider
// edits the operator goes to the dedicated Providers page. We do show
// live latency / error stats here when live mode is on so the
// operator can spot a misbehaving upstream without leaving the canvas.

import { useMemo } from "react";
import { useRouteGraphStore } from "@/store/routeGraphStore";
import type { ProviderNodeData } from "../utils/buildGraph";

interface Props {
  data: ProviderNodeData;
  nodeId: string;
}

export function ProviderPanel({ data, nodeId }: Props) {
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const liveStats = useRouteGraphStore((s) => s.liveStats);
  const edges = useRouteGraphStore((s) => s.edges);

  // Aggregate inbound edge stats so the operator sees one summary
  // line per provider regardless of how many routes feed it.
  const summary = useMemo(() => {
    const inbound = edges.filter((e) => e.target === nodeId);
    let rps = 0;
    let weightedErr = 0;
    let n = 0;
    for (const e of inbound) {
      const s = liveStats[e.id];
      if (!s) continue;
      rps += s.rps;
      weightedErr += s.errorRate * s.rps;
      n++;
    }
    return {
      rps,
      errorRate: rps > 0 ? weightedErr / rps : 0,
      hasData: n > 0,
    };
  }, [liveStats, edges, nodeId]);

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Provider</h3>
        <p className="text-xs text-muted-foreground">
          Concrete upstream endpoint (read-only here).
        </p>
      </div>

      <dl className="space-y-2 text-xs">
        <div>
          <dt className="text-muted-foreground">Provider</dt>
          <dd className="font-mono">{data.provider}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">Model</dt>
          <dd className="font-mono">{data.model}</dd>
        </div>
        {data.config?.kind && (
          <div>
            <dt className="text-muted-foreground">Kind</dt>
            <dd className="font-mono">{data.config.kind}</dd>
          </div>
        )}
        {data.config?.base_url && (
          <div>
            <dt className="text-muted-foreground">Base URL</dt>
            <dd className="font-mono break-all">{data.config.base_url}</dd>
          </div>
        )}
      </dl>

      {liveMode && (
        <div className="rounded-md border border-border/60 bg-muted/30 p-3 space-y-1.5 text-xs">
          <div className="text-[10px] uppercase tracking-wide text-muted-foreground">
            Live (last 30s)
          </div>
          {summary.hasData ? (
            <>
              <div className="flex justify-between">
                <span>Throughput</span>
                <span className="font-mono">{summary.rps.toFixed(1)} req/s</span>
              </div>
              <div className="flex justify-between">
                <span>Error rate</span>
                <span className="font-mono">
                  {(summary.errorRate * 100).toFixed(2)}%
                </span>
              </div>
            </>
          ) : (
            <div className="text-muted-foreground">no traffic</div>
          )}
        </div>
      )}
    </div>
  );
}
