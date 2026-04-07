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

interface RouteGraphState {
  nodes: Node[];
  edges: Edge[];
  selectedNodeId: string | null;
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
  toggleLiveMode: () => void;
  updateLiveStats: (stats: Record<string, EdgeStat>) => void;
}

export const useRouteGraphStore = create<RouteGraphState>((set) => ({
  nodes: [],
  edges: [],
  selectedNodeId: null,
  liveMode: false,
  liveStats: {},

  setNodes: (nodes) => set({ nodes }),
  setEdges: (edges) => set({ edges }),
  setGraph: (nodes, edges) => set({ nodes, edges }),
  selectNode: (id) => set({ selectedNodeId: id }),
  toggleLiveMode: () => set((s) => ({ liveMode: !s.liveMode })),
  updateLiveStats: (stats) => set({ liveStats: stats }),
}));
