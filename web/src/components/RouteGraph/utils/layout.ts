// layout.ts — dagre-powered auto layout for the route graph.
//
// React Flow does not ship with a layout algorithm; it just renders
// whatever (x, y) coordinates you give it. Dagre is the standard
// pairing for left-to-right DAGs and is what the React Flow team
// recommends in their own examples. We wrap it in a single utility so
// the rest of the codebase never imports dagre directly.

import dagre from "dagre";
import type { Edge, Node } from "@xyflow/react";

// NODE_WIDTH / NODE_HEIGHT are the *layout* dimensions, used by dagre
// to space nodes apart. They do not have to match the rendered size
// exactly — they just need to be large enough that nodes never overlap
// after layout. The visual width is set in each node component.
const NODE_WIDTH = 240;
const NODE_HEIGHT = 96;

// getLayoutedElements runs dagre over the (nodes, edges) pair and
// returns a new nodes array with `position` populated. Edges pass
// through unchanged because dagre only computes node positions —
// React Flow draws the actual edge paths.
//
// rankdir = "LR" gives us a horizontal flow which matches the mental
// model "request comes in on the left, ends at a provider on the
// right". ranksep / nodesep are tuned for our node widths to leave
// enough room for the longest labels and the route fanout from a
// virtual model node.
export function getLayoutedElements(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "LR", ranksep: 110, nodesep: 50, marginx: 24, marginy: 24 });

  nodes.forEach((n) => g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT }));
  edges.forEach((e) => g.setEdge(e.source, e.target));

  dagre.layout(g);

  return {
    nodes: nodes.map((n) => {
      const { x, y } = g.node(n.id);
      // dagre returns the node *centre*; React Flow expects the
      // top-left corner, so we shift by half-width/height.
      return {
        ...n,
        position: { x: x - NODE_WIDTH / 2, y: y - NODE_HEIGHT / 2 },
      };
    }),
    edges,
  };
}

// LAYOUT_STORAGE is the localStorage key under which we persist the
// user's manual node positions. It is keyed by node id so layout
// changes (new nodes added) gracefully fall back to dagre for any node
// the user has never moved.
export const LAYOUT_STORAGE = "fluxa-route-graph-positions";

export type StoredPositions = Record<string, { x: number; y: number }>;

export function loadStoredPositions(): StoredPositions {
  try {
    const raw = localStorage.getItem(LAYOUT_STORAGE);
    return raw ? (JSON.parse(raw) as StoredPositions) : {};
  } catch {
    return {};
  }
}

export function saveStoredPositions(positions: StoredPositions): void {
  try {
    localStorage.setItem(LAYOUT_STORAGE, JSON.stringify(positions));
  } catch {
    // Storage quota errors are silently ignored — the layout will
    // simply re-run from dagre on next reload.
  }
}

// applyStoredPositions overlays any saved manual positions on top of
// the dagre-laid-out nodes. This is the function the main component
// calls right after running getLayoutedElements: dagre gives sensible
// defaults, then user overrides win.
export function applyStoredPositions(nodes: Node[]): Node[] {
  const stored = loadStoredPositions();
  return nodes.map((n) =>
    stored[n.id] ? { ...n, position: stored[n.id] } : n,
  );
}
