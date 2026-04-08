// routeGraphStore.ts — Zustand store backing the visual route flow
// editor. The graph is rebuilt from the admin API on every refresh, so
// the store keeps only the *currently materialised* nodes / edges plus
// transient UI state (which node is selected, whether live mode is on,
// the most recent live stats sample). Persistent route configuration
// lives on the server, never here.
//
// We use Zustand instead of useState in the top-level component for two
// reasons:
//   1. The side panel sits in a sibling subtree of ReactFlow but needs
//      to react to whatever node the user just clicked on the canvas;
//      lifting that into a context-free store keeps the prop drilling
//      to zero.
//   2. The toolbar mutates the same state ("toggle live mode", "reset
//      layout") and benefits from the same global hook.

import { create } from "zustand";
import type { Edge, Node } from "@xyflow/react";

// EdgeStat is the per-edge sample we get back from /admin/stats/edges.
// rps is requests-per-second, errorRate is a 0..1 fraction. Both are
// rolling 30-second windows on the server side.
export interface EdgeStat {
  rps: number;
  errorRate: number;
}

// CreatingKind tags which create-flow the side panel is currently
// hosting. The side panel renders an empty form for the matching
// type when this is set, and the toolbar's "+" buttons set it.
export type CreatingKind = "regexRoute" | "virtualModel" | null;

interface RouteGraphState {
  nodes: Node[];
  edges: Edge[];
  selectedNodeId: string | null;
  // creatingKind and selectedNodeId are mutually exclusive: opening
  // one closes the other. The side panel checks both to decide
  // whether to render and what to render.
  creatingKind: CreatingKind;
  // draftNodeId is the id of the placeholder node currently sitting
  // on the canvas while the operator fills out the create form. The
  // node lives in the regular `nodes` array (so React Flow renders
  // it like any other) but with `data.draft = true` so the node
  // components give it a dashed "未保存" treatment. The form state
  // itself is owned by the side panel — the draft node is purely
  // a visual placeholder and is replaced on save when load()
  // rebuilds the graph from the server.
  draftNodeId: string | null;
  liveMode: boolean;
  // liveStats is keyed by edge id so the WeightedEdge / RouteEdge
  // components can do an O(1) lookup during render. Provider node
  // health is derived from this same map (we aggregate the inbound
  // edges' error rates in ProviderNode).
  liveStats: Record<string, EdgeStat>;

  setNodes: (nodes: Node[]) => void;
  setEdges: (edges: Edge[]) => void;
  setGraph: (nodes: Node[], edges: Edge[]) => void;
  selectNode: (id: string | null) => void;
  // startCreate registers a create intent and (optionally) the id of
  // the draft node the caller already inserted into the canvas. Pass
  // null to clear without touching the nodes array.
  startCreate: (kind: CreatingKind, draftNodeId?: string | null) => void;
  toggleLiveMode: () => void;
  updateLiveStats: (stats: Record<string, EdgeStat>) => void;
}

export const useRouteGraphStore = create<RouteGraphState>((set) => ({
  nodes: [],
  edges: [],
  selectedNodeId: null,
  creatingKind: null,
  draftNodeId: null,
  // Live mode is on by default so the canvas immediately shows
  // animated traffic flow when an operator opens the page. Without
  // this the page would look static and the routing topology would
  // be hard to read at a glance — the animated dashes are the single
  // strongest visual cue that traffic flows left-to-right.
  liveMode: true,
  liveStats: {},

  setNodes: (nodes) => set({ nodes }),
  setEdges: (edges) => set({ edges }),
  setGraph: (nodes, edges) => set({ nodes, edges }),
  // selectNode and startCreate are mutually exclusive — opening one
  // mode dismisses the other so the side panel never holds two open
  // intents at once.
  selectNode: (id) => set({ selectedNodeId: id, creatingKind: null }),
  startCreate: (kind, draftNodeId = null) =>
    set({ creatingKind: kind, draftNodeId, selectedNodeId: null }),
  toggleLiveMode: () => set((s) => ({ liveMode: !s.liveMode })),
  updateLiveStats: (stats) => set({ liveStats: stats }),
}));
