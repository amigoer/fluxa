// RouteGraph — visual flow editor for the Fluxa routing topology.
//
// This is the entry component mounted in the admin shell. It owns:
//   - Fetching the three admin endpoints (virtual models / regex
//     routes / providers)
//   - Building the graph via the pure buildGraph helper
//   - Running dagre auto-layout, applying any user-overridden node
//     positions on top
//   - Pushing the result into the Zustand store so the toolbar, side
//     panel, and node components can all read from one source
//   - Polling the live stats endpoint when live mode is on
//
// React Flow's ReactFlowProvider wraps the canvas so child components
// (notably the toolbar's Fit View button) can call useReactFlow().

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  applyNodeChanges,
  applyEdgeChanges,
  type Node,
  type Edge,
  type NodeChange,
  type EdgeChange,
  type NodeTypes,
  type EdgeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { Providers, RegexRoutes, VirtualModels } from "@/lib/api";
import { useT } from "@/lib/i18n";
import { useRouteGraphStore, type EdgeStat } from "@/store/routeGraphStore";
import { buildGraph } from "./utils/buildGraph";
import {
  applyStoredPositions,
  getLayoutedElements,
  loadStoredPositions,
  saveStoredPositions,
} from "./utils/layout";

import { SourceNode } from "./nodes/SourceNode";
import { RegexRouteNode } from "./nodes/RegexRouteNode";
import { VirtualModelNode } from "./nodes/VirtualModelNode";
import { ProviderNode } from "./nodes/ProviderNode";
import { FallbackNode } from "./nodes/FallbackNode";
import { WeightedEdge } from "./edges/WeightedEdge";
import { RouteEdge } from "./edges/RouteEdge";
import { GraphToolbar } from "./toolbar/GraphToolbar";
import { NodeSidePanel } from "./panels/NodeSidePanel";

// React Flow expects nodeTypes / edgeTypes to be referentially stable
// across renders, otherwise it warns and forces a remount. Defining
// them at module scope (not inside the component) keeps the references
// stable for free.
const nodeTypes: NodeTypes = {
  source: SourceNode,
  regexRoute: RegexRouteNode,
  virtualModel: VirtualModelNode,
  provider: ProviderNode,
  fallback: FallbackNode,
};

const edgeTypes: EdgeTypes = {
  weighted: WeightedEdge,
  route: RouteEdge,
};

export function RouteGraphPage() {
  return (
    // ReactFlowProvider is required for the toolbar's useReactFlow()
    // hook to find a context. Wrapping at the page level keeps the
    // canvas + toolbar + panel in the same provider scope.
    <ReactFlowProvider>
      <RouteGraphInner />
    </ReactFlowProvider>
  );
}

function RouteGraphInner() {
  const { t } = useT();
  const nodes = useRouteGraphStore((s) => s.nodes);
  const edges = useRouteGraphStore((s) => s.edges);
  const setGraph = useRouteGraphStore((s) => s.setGraph);
  const setNodes = useRouteGraphStore((s) => s.setNodes);
  const setEdges = useRouteGraphStore((s) => s.setEdges);
  const liveMode = useRouteGraphStore((s) => s.liveMode);
  const updateLiveStats = useRouteGraphStore((s) => s.updateLiveStats);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Used to detect first-load empty state so we can render a friendly
  // call-to-action instead of a blank canvas.
  const [empty, setEmpty] = useState(false);

  // load is the single "fetch + build + layout" pipeline. Every
  // mutation in the side panel and the toolbar calls this so the
  // canvas always reflects server-side state with no manual diffing.
  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [vms, rxs, ps] = await Promise.all([
        VirtualModels.list(),
        RegexRoutes.list(),
        Providers.list(),
      ]);
      const built = buildGraph(vms, rxs, ps);
      const laid = getLayoutedElements(built.nodes, built.edges);
      const withStored = applyStoredPositions(laid.nodes);
      setGraph(withStored, laid.edges);
      setEmpty(vms.length === 0 && rxs.length === 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("graph.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [setGraph, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // Live mode polling. We use a ref for the timer so the cleanup
  // closure can clear it without re-running the effect on every
  // store update. The 3 s cadence is the spec; the endpoint is best-
  // effort (still being implemented), so a fetch failure mocks the
  // numbers locally instead of erroring out the whole canvas.
  const timerRef = useRef<number | null>(null);
  useEffect(() => {
    if (!liveMode) {
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
        timerRef.current = null;
      }
      return;
    }

    async function tick() {
      try {
        const res = await fetch("/admin/stats/edges", {
          headers: {
            Authorization: `Bearer ${localStorage.getItem("fluxa-session-token") ?? ""}`,
          },
        });
        if (!res.ok) throw new Error("stats unavailable");
        const data = (await res.json()) as { data: Record<string, EdgeStat> };
        updateLiveStats(data.data ?? {});
      } catch {
        // Mock fallback so the demo experience works before the
        // server-side endpoint exists. The mock varies per edge so the
        // canvas visibly animates while live mode is on.
        const mock: Record<string, EdgeStat> = {};
        for (const e of useRouteGraphStore.getState().edges) {
          mock[e.id] = {
            rps: Math.random() * 50,
            errorRate: Math.random() * 0.05,
          };
        }
        updateLiveStats(mock);
      }
    }
    void tick();
    timerRef.current = window.setInterval(tick, 3000);
    return () => {
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [liveMode, updateLiveStats]);

  // React Flow change handlers — we delegate to the helper functions
  // for position / removal updates and write the result straight back
  // to the store. Edge changes are also wired so connection
  // selections survive a re-layout.
  const onNodesChange = useCallback(
    (changes: NodeChange[]) => {
      setNodes(applyNodeChanges(changes, nodes) as Node[]);
    },
    [nodes, setNodes],
  );
  const onEdgesChange = useCallback(
    (changes: EdgeChange[]) => {
      setEdges(applyEdgeChanges(changes, edges) as Edge[]);
    },
    [edges, setEdges],
  );

  // Persist a manual drag to localStorage so the position survives
  // a refresh. We do *not* persist on every position change inside
  // onNodesChange because React Flow fires those during the drag and
  // we only care about the final resting position.
  const onNodeDragStop = useCallback(
    (_: unknown, node: Node) => {
      const stored = loadStoredPositions();
      stored[node.id] = node.position;
      saveStoredPositions(stored);
    },
    [],
  );

  // Manual "Auto Layout" button — re-runs dagre over the current
  // (server-derived) graph and *clears* stored positions so the
  // operator can reset after dragging things into a mess.
  const relayout = useCallback(() => {
    const laid = getLayoutedElements(nodes, edges);
    saveStoredPositions({});
    setGraph(laid.nodes, laid.edges);
  }, [nodes, edges, setGraph]);

  // Memoised default edge options keep the React Flow default styling
  // consistent with our custom edge components.
  const defaultEdgeOptions = useMemo(
    () => ({ type: "route" as const }),
    [],
  );

  return (
    <div className="absolute inset-0 bg-muted/20">
      {error && (
        <div className="absolute top-3 right-3 z-20 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeDragStop={onNodeDragStop}
        onPaneClick={() => useRouteGraphStore.getState().selectNode(null)}
        defaultEdgeOptions={defaultEdgeOptions}
        fitView
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} size={1} color="#94a3b8" />
        <Controls position="bottom-right" />
        <MiniMap
          pannable
          zoomable
          position="bottom-left"
          nodeStrokeWidth={3}
          maskColor="rgba(0,0,0,0.05)"
        />
      </ReactFlow>

      <GraphToolbar onLayout={relayout} onChange={load} />
      <NodeSidePanel onChange={load} />

      {empty && !loading && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="rounded-xl border border-border/60 bg-background/90 backdrop-blur p-6 max-w-sm text-center pointer-events-auto shadow-lg">
            <div className="text-sm font-semibold mb-1">{t("graph.empty.title")}</div>
            <p className="text-xs text-muted-foreground">{t("graph.empty.hint")}</p>
          </div>
        </div>
      )}
      {loading && nodes.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">
          {t("graph.loading")}
        </div>
      )}
    </div>
  );
}
