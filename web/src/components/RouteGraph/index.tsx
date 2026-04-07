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
  type Connection,
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
import type {
  ProviderNodeData,
  RegexNodeData,
  VirtualModelNodeData,
} from "./utils/buildGraph";
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
  const startCreate = useRouteGraphStore((s) => s.startCreate);
  const updateDraftRegex = useRouteGraphStore((s) => s.updateDraftRegex);
  const updateDraftVirtual = useRouteGraphStore((s) => s.updateDraftVirtual);

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

  // onStartCreate — toolbar entry point. Inserts a draft node onto
  // the canvas (positioned a comfortable offset from the source so
  // it lands somewhere visible without disturbing the existing
  // layout) and wires a source→draft edge so the new card is part
  // of the topology from the moment it appears. Then it pings
  // store.startCreate so the side panel knows to render the create
  // form bound to this draft. The form mutates the draft node's
  // data live via updateDraftRegex / updateDraftVirtual.
  const onStartCreate = useCallback(
    (kind: "regexRoute" | "virtualModel") => {
      // Clean up any previous draft so consecutive clicks of the
      // toolbar buttons swap drafts cleanly instead of stacking.
      const prior = useRouteGraphStore.getState().draftNodeId;
      const baseNodes = prior
        ? nodes.filter((n) => n.id !== prior)
        : nodes;
      const baseEdges = prior
        ? edges.filter((e) => e.source !== prior && e.target !== prior)
        : edges;

      // Pick a spot offset from the source node. Falls back to the
      // origin if there is no source node yet (shouldn't happen but
      // keeps the function total).
      const sourceNode = baseNodes.find((n) => n.id === "source");
      const offsetX = sourceNode ? sourceNode.position.x + 320 : 320;
      const offsetY = sourceNode ? sourceNode.position.y - 60 : 0;

      const id =
        kind === "regexRoute"
          ? `draft-regex-${Date.now()}`
          : `draft-virtual-${Date.now()}`;

      let draftNode: Node;
      if (kind === "regexRoute") {
        draftNode = {
          id,
          type: "regexRoute",
          position: { x: offsetX, y: offsetY },
          data: {
            route: {
              pattern: "",
              priority: 100,
              target_type: "virtual",
              target_model: "",
              provider: "",
              description: "",
              enabled: true,
            },
            draft: true,
          } as unknown as Record<string, unknown>,
        };
      } else {
        draftNode = {
          id,
          type: "virtualModel",
          position: { x: offsetX, y: offsetY },
          data: {
            model: {
              name: "",
              description: "",
              enabled: true,
              routes: [
                {
                  weight: 100,
                  target_type: "real",
                  target_model: "",
                  provider: "",
                  enabled: true,
                },
              ],
            },
            colors: ["#534AB7"],
            draft: true,
          } as unknown as Record<string, unknown>,
        };
      }

      // The source→draft edge uses the same labelKind we'd assign
      // after the rule was saved (priority for regex, direct for VM)
      // so the visual is identical pre/post save — only the dashed
      // border on the draft card itself signals "unsaved".
      const draftEdge: Edge = {
        id: `e:source->${id}`,
        source: "source",
        target: id,
        type: "route",
        data: {
          labelKind: kind === "regexRoute" ? "priority" : "direct",
          priority: 100,
        },
      };

      setGraph([...baseNodes, draftNode], [...baseEdges, draftEdge]);
      startCreate(kind, id);
    },
    [nodes, edges, setGraph, startCreate],
  );

  // onCancelCreate — invoked by the side panel when the operator
  // dismisses the create form without saving. Removes the draft
  // node + its source edge from the canvas and clears the create
  // intent in the store.
  const onCancelCreate = useCallback(() => {
    const draftId = useRouteGraphStore.getState().draftNodeId;
    if (!draftId) {
      startCreate(null);
      return;
    }
    setGraph(
      nodes.filter((n) => n.id !== draftId),
      edges.filter((e) => e.source !== draftId && e.target !== draftId),
    );
    startCreate(null);
  }, [nodes, edges, setGraph, startCreate]);

  // onConnect — wires the user's manual handle-to-handle drag to a
  // backend mutation so the new connection actually persists. We
  // interpret the drag as "rewire the source-side route's target":
  //
  //   VirtualModel handle "route-N" → Provider/VM
  //     -> route N of the VM is reassigned to the new target.
  //
  //   RegexRoute handle → Provider/VM
  //     -> the regex's target_model / provider is reassigned.
  //
  // Anything else (drags from source/fallback, drops on source/
  // fallback) is rejected silently — those nodes are synthetic and
  // do not correspond to mutable rows in the store.
  //
  // We read the store directly inside the callback (rather than from
  // the closed-over `nodes`) so a stale render of nodes from the
  // hook subscription cannot fool us into wiring against an old
  // snapshot.
  const onConnect = useCallback(
    async (conn: Connection) => {
      if (!conn.source || !conn.target || conn.source === conn.target) return;
      const currentNodes = useRouteGraphStore.getState().nodes;
      const sourceNode = currentNodes.find((n) => n.id === conn.source);
      const targetNode = currentNodes.find((n) => n.id === conn.target);
      if (!sourceNode || !targetNode) return;

      // Compute the new target descriptor based on what the user
      // dropped on. Both branches return the same shape so the
      // upstream switch can stay flat.
      let newTarget:
        | { target_type: "real"; target_model: string; provider: string }
        | { target_type: "virtual"; target_model: string; provider: "" }
        | null = null;
      if (targetNode.type === "provider") {
        const p = targetNode.data as unknown as ProviderNodeData;
        newTarget = {
          target_type: "real",
          target_model: p.model,
          provider: p.provider,
        };
      } else if (targetNode.type === "virtualModel") {
        const v = targetNode.data as unknown as VirtualModelNodeData;
        newTarget = {
          target_type: "virtual",
          target_model: v.model.name,
          provider: "",
        };
      } else {
        return; // dropping on source / fallback / regex is meaningless
      }

      // Drafts are intercepted before any network call: dragging
      // from a not-yet-saved draft node should mutate the *form*
      // bound to that draft, not POST half-built data to the
      // server. The side panel observes the same store update and
      // re-renders the input fields with the dropped target.
      const isDraft = sourceNode.id.startsWith("draft-");
      if (isDraft) {
        if (sourceNode.type === "virtualModel") {
          const v = sourceNode.data as unknown as VirtualModelNodeData;
          // Reject the same self-loop as the persisted-edit branch.
          if (
            newTarget.target_type === "virtual" &&
            newTarget.target_model === v.model.name
          ) {
            return;
          }
          // Idx defaults to 0 because a brand-new draft VM has
          // exactly one seeded route. If the operator has already
          // added more routes via the form they can drop into the
          // matching handle and we honour the route-N convention.
          const idxStr = (conn.sourceHandle ?? "route-0").replace(
            /^route-/,
            "",
          );
          const idx = parseInt(idxStr, 10);
          if (Number.isNaN(idx) || idx < 0 || idx >= v.model.routes.length) {
            return;
          }
          updateDraftVirtual({
            ...v.model,
            routes: v.model.routes.map((r, i) =>
              i === idx ? { ...r, ...newTarget } : r,
            ),
          });
          return;
        }
        if (sourceNode.type === "regexRoute") {
          const r = (sourceNode.data as unknown as RegexNodeData).route;
          updateDraftRegex({
            ...r,
            target_type: newTarget.target_type,
            target_model: newTarget.target_model,
            provider: newTarget.provider,
          });
          return;
        }
      }

      try {
        if (sourceNode.type === "virtualModel") {
          // Rewire route N of this VM. The handle id encodes the
          // route index ("route-2" -> idx 2). If the user dragged
          // from a non-route handle (defensive — there are no other
          // source handles on a VM today) we bail.
          const v = sourceNode.data as unknown as VirtualModelNodeData;
          const idxStr = (conn.sourceHandle ?? "").replace(/^route-/, "");
          const idx = parseInt(idxStr, 10);
          if (Number.isNaN(idx) || idx < 0 || idx >= v.model.routes.length) {
            return;
          }
          // Reject self-loops: a VM route pointing back at the same VM
          // would create a runtime cycle the resolver caps at depth 5
          // and confusing topology.
          if (
            newTarget.target_type === "virtual" &&
            newTarget.target_model === v.model.name
          ) {
            return;
          }
          const updated = {
            ...v.model,
            routes: v.model.routes.map((r, i) =>
              i === idx ? { ...r, ...newTarget } : r,
            ),
          };
          await VirtualModels.upsert(updated);
        } else if (sourceNode.type === "regexRoute") {
          const r = (sourceNode.data as unknown as RegexNodeData).route;
          if (!r.id) return;
          await RegexRoutes.update(r.id, {
            ...r,
            target_type: newTarget.target_type,
            target_model: newTarget.target_model,
            provider: newTarget.provider,
          });
        } else {
          return; // source / fallback can't originate edges in our model
        }
        await load();
      } catch (err) {
        setError(err instanceof Error ? err.message : t("graph.errors.save"));
      }
    },
    [load, t, updateDraftRegex, updateDraftVirtual],
  );

  // isValidConnection — runs while the operator is dragging, so
  // ReactFlow can show a green / red drop affordance on each handle.
  // The rules mirror what onConnect actually accepts: VM/regex
  // sources, provider/VM targets, no self-loops.
  const isValidConnection = useCallback((conn: Edge | Connection) => {
    if (!conn.source || !conn.target || conn.source === conn.target) {
      return false;
    }
    const currentNodes = useRouteGraphStore.getState().nodes;
    const s = currentNodes.find((n) => n.id === conn.source);
    const t = currentNodes.find((n) => n.id === conn.target);
    if (!s || !t) return false;
    const validSource = s.type === "virtualModel" || s.type === "regexRoute";
    const validTarget = t.type === "provider" || t.type === "virtualModel";
    return validSource && validTarget;
  }, []);

  // layoutAnimRef holds the RAF handle of an in-flight relayout
  // tween. Storing it on a ref (rather than React state) avoids a
  // re-render on every frame and lets a second click of "Auto
  // Layout" cancel the first cleanly instead of two RAF loops
  // fighting each other for the node positions.
  const layoutAnimRef = useRef<number | null>(null);

  // Manual "Auto Layout" button — re-runs dagre over the current
  // (server-derived) graph and *clears* stored positions so the
  // operator can reset after dragging things into a mess.
  //
  // The transition is animated rather than instantaneous: jumping
  // nodes hundreds of pixels in one frame is jarring and makes it
  // hard for the operator to mentally track which node went where.
  // We capture each node's current position, compute its dagre
  // target, then tween via requestAnimationFrame with an ease-out
  // cubic so the motion decelerates into place. Edges follow
  // automatically because React Flow re-derives edge paths from
  // node positions on every render.
  const relayout = useCallback(() => {
    if (layoutAnimRef.current !== null) {
      cancelAnimationFrame(layoutAnimRef.current);
      layoutAnimRef.current = null;
    }
    const laid = getLayoutedElements(nodes, edges);
    saveStoredPositions({});
    // Push the freshly-laid edges once up front; edges do not need
    // to be tweened because their bezier paths are derived from the
    // (animating) node positions on every React Flow render.
    setEdges(laid.edges);

    // Snapshot starting positions so we can interpolate against the
    // *original* values for the whole tween, not the previous frame.
    const startPositions = new Map<string, { x: number; y: number }>(
      nodes.map((n) => [n.id, { ...n.position }]),
    );

    const duration = 500;
    const easeOutCubic = (k: number) => 1 - Math.pow(1 - k, 3);
    const startTime = performance.now();

    function tick() {
      const elapsed = performance.now() - startTime;
      const k = Math.min(1, elapsed / duration);
      const eased = easeOutCubic(k);
      const animated = laid.nodes.map((n) => {
        const start = startPositions.get(n.id);
        if (!start) return n;
        return {
          ...n,
          position: {
            x: start.x + (n.position.x - start.x) * eased,
            y: start.y + (n.position.y - start.y) * eased,
          },
        };
      });
      setNodes(animated);
      if (k < 1) {
        layoutAnimRef.current = requestAnimationFrame(tick);
      } else {
        layoutAnimRef.current = null;
      }
    }
    layoutAnimRef.current = requestAnimationFrame(tick);
  }, [nodes, edges, setNodes, setEdges]);

  // Cancel any in-flight layout tween when the page unmounts so we
  // never schedule a setNodes against a torn-down component.
  useEffect(() => {
    return () => {
      if (layoutAnimRef.current !== null) {
        cancelAnimationFrame(layoutAnimRef.current);
      }
    };
  }, []);

  // Memoised default edge options keep the React Flow default styling
  // consistent with our custom edge components.
  const defaultEdgeOptions = useMemo(
    () => ({ type: "route" as const }),
    [],
  );

  return (
    <div className="absolute inset-0 bg-[#f8f8f6] dark:bg-zinc-950">
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
        onConnect={onConnect}
        isValidConnection={isValidConnection}
        onNodeDragStop={onNodeDragStop}
        onPaneClick={() => useRouteGraphStore.getState().selectNode(null)}
        defaultEdgeOptions={defaultEdgeOptions}
        connectionLineStyle={{ stroke: "#7F77DD", strokeWidth: 2 }}
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

      <GraphToolbar
        onLayout={relayout}
        onChange={load}
        onStartCreate={onStartCreate}
      />
      <NodeSidePanel onChange={load} onCancelCreate={onCancelCreate} />

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
