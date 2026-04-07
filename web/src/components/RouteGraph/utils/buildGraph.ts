// buildGraph.ts — converts a snapshot of admin API data into the
// (nodes, edges) shape that React Flow consumes.
//
// The graph has five distinct node types and a strict topological
// shape:
//
//   SourceNode (1)  ─┬─►  RegexRouteNode  ─►  VirtualModelNode  ─►  ProviderNode
//                    │                    └►  ProviderNode
//                    └─►  FallbackNode
//
// VirtualModelNodes can also point at *other* virtual model nodes
// (nested aliasing) — buildGraph handles that case by emitting a
// virtual→virtual edge instead of a virtual→provider edge.
//
// We deliberately compute the graph from a single immutable snapshot
// rather than mutating an existing one. The cost is rebuilding ~50
// nodes on every refresh (cheap), and the benefit is no stale-node /
// stale-edge bugs after a delete-then-recreate cycle.

import type { Edge, Node } from "@xyflow/react";
import type {
  Provider,
  RegexRoute,
  VirtualModel,
  VirtualModelRoute,
} from "@/lib/api";

// Stable id helpers — keeping the prefix scheme readable means a
// failing build (or a logged React Flow warning) immediately tells you
// which node type the offending id belongs to.
const SOURCE_ID = "source";
const FALLBACK_ID = "fallback";
const regexId = (r: RegexRoute) => `regex:${r.id ?? r.pattern}`;
const vmId = (name: string) => `vm:${name}`;
const providerId = (provider: string, model: string) =>
  `provider:${provider}:${model}`;

// Node data payloads. Each custom node component declares its own
// `data` shape; we keep them all in one file so the build function and
// the components agree on field names.
export interface SourceNodeData {
  label: string;
}

export interface RegexNodeData {
  route: RegexRoute;
}

export interface VirtualModelNodeData {
  model: VirtualModel;
  // colors aligned 1:1 with model.routes so the donut and the side
  // panel can share a palette without recomputing it on every render.
  colors: string[];
}

export interface ProviderNodeData {
  provider: string;
  model: string;
  config?: Provider;
}

export interface FallbackNodeData {
  label: string;
}

// Edge data — the same payload covers both edge types; the React Flow
// edge component picks out only the fields it cares about.
export interface RouteEdgeData {
  weight?: number;
  weightPct?: number;
  // labelKind tags the edge so the renderer can pull the right
  // localized string at draw time. We store the *kind* (plus the raw
  // priority for the source-side edge) instead of a pre-formatted
  // English label so the same buildGraph snapshot renders correctly
  // in any locale without a rebuild.
  labelKind?: "priority" | "matched" | "noMatch";
  priority?: number;
}

// DONUT_PALETTE is the colour ring used for VirtualModelNode segments.
// Picked to stay legible on both light and dark backgrounds and to
// avoid the red/green pair that we reserve for live-mode health dots.
const DONUT_PALETTE = [
  "#8b5cf6", // violet-500
  "#06b6d4", // cyan-500
  "#f59e0b", // amber-500
  "#ec4899", // pink-500
  "#10b981", // emerald-500
  "#3b82f6", // blue-500
  "#f97316", // orange-500
  "#a855f7", // purple-500
];

// buildGraph is the workhorse: take three lists, return the full
// (nodes, edges) snapshot. Pure function — no side effects, no I/O —
// which makes it trivial to unit test if/when we add coverage.
export function buildGraph(
  virtualModels: VirtualModel[],
  regexRoutes: RegexRoute[],
  providers: Provider[],
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  // 1. SourceNode — always exactly one, anchored at the left edge of
  //    the graph. Acts as the conceptual "request enters here" marker.
  nodes.push({
    id: SOURCE_ID,
    type: "source",
    position: { x: 0, y: 0 },
    data: { label: "Incoming Request" } satisfies SourceNodeData,
  });

  // 2. VirtualModelNodes — one per configured virtual model. We index
  //    them by name so the regex-route loop below can wire targets
  //    without a second pass over the list.
  const vmIndex = new Map<string, VirtualModel>();
  for (const vm of virtualModels) {
    vmIndex.set(vm.name, vm);
    const colors = vm.routes.map((_, i) => DONUT_PALETTE[i % DONUT_PALETTE.length]);
    nodes.push({
      id: vmId(vm.name),
      type: "virtualModel",
      position: { x: 0, y: 0 },
      data: { model: vm, colors } satisfies VirtualModelNodeData,
    });
  }

  // 3. ProviderNodes — one per *unique* (provider, model) tuple
  //    referenced anywhere in the graph (regex routes or virtual model
  //    routes). We do not draw a provider for every model the provider
  //    advertises in `models[]` because that would explode the canvas
  //    with nodes the user never actually routes to.
  const providerIndex = new Map<string, Provider>();
  for (const p of providers) providerIndex.set(p.name, p);

  const providerNodeIds = new Set<string>();
  function ensureProviderNode(provider: string, model: string): string {
    const id = providerId(provider, model);
    if (!providerNodeIds.has(id)) {
      providerNodeIds.add(id);
      nodes.push({
        id,
        type: "provider",
        position: { x: 0, y: 0 },
        data: {
          provider,
          model,
          config: providerIndex.get(provider),
        } satisfies ProviderNodeData,
      });
    }
    return id;
  }

  // 4. RegexRouteNodes — emitted in priority order (lowest number =
  //    highest priority). The visual stacking matches the actual
  //    runtime evaluation order, which is the single most important
  //    thing the operator wants to verify on this screen.
  const sortedRegex = [...regexRoutes].sort(
    (a, b) => (a.priority ?? 100) - (b.priority ?? 100),
  );
  for (const r of sortedRegex) {
    const id = regexId(r);
    nodes.push({
      id,
      type: "regexRoute",
      position: { x: 0, y: 0 },
      data: { route: r } satisfies RegexNodeData,
    });
    edges.push({
      id: `e:${SOURCE_ID}->${id}`,
      source: SOURCE_ID,
      target: id,
      type: "route",
      data: {
        labelKind: "priority",
        priority: r.priority ?? 100,
      } satisfies RouteEdgeData,
    });

    // Wire each regex node to its target. Real targets land on the
    // provider node (creating it on demand if needed); virtual targets
    // land on the matching VirtualModelNode.
    if (r.target_type === "real" && r.provider) {
      const pid = ensureProviderNode(r.provider, r.target_model);
      edges.push({
        id: `e:${id}->${pid}`,
        source: id,
        target: pid,
        type: "route",
        data: { labelKind: "matched" } satisfies RouteEdgeData,
      });
    } else if (r.target_type === "virtual" && vmIndex.has(r.target_model)) {
      edges.push({
        id: `e:${id}->${vmId(r.target_model)}`,
        source: id,
        target: vmId(r.target_model),
        type: "route",
        data: { labelKind: "matched" } satisfies RouteEdgeData,
      });
    }
  }

  // 5. FallbackNode — the terminal "no rule matched, pass the model
  //    name through unchanged" branch. Always present so the canvas
  //    visually communicates that there is *always* a default path,
  //    even if the user has not configured any regex routes yet.
  nodes.push({
    id: FALLBACK_ID,
    type: "fallback",
    position: { x: 0, y: 0 },
    data: { label: "Passthrough" } satisfies FallbackNodeData,
  });
  edges.push({
    id: `e:${SOURCE_ID}->${FALLBACK_ID}`,
    source: SOURCE_ID,
    target: FALLBACK_ID,
    type: "route",
    data: { labelKind: "noMatch" } satisfies RouteEdgeData,
  });

  // 6. Virtual model fanout edges. We compute total weight per VM
  //    once so the per-edge percentage is honest even when the
  //    operator has weights that do not sum to 100. The
  //    handle id (`route-${idx}`) lets the VirtualModelNode render one
  //    output handle per route on its right edge — without per-route
  //    handles, dagre routes every fanout edge through a single point
  //    and the layout looks crowded.
  for (const vm of virtualModels) {
    const total = vm.routes.reduce(
      (acc: number, r: VirtualModelRoute) => acc + (r.weight || 0),
      0,
    );
    vm.routes.forEach((route, idx) => {
      const sourceHandle = `route-${idx}`;
      const pct = total > 0 ? Math.round(((route.weight || 0) / total) * 100) : 0;
      if (route.target_type === "real" && route.provider) {
        const pid = ensureProviderNode(route.provider, route.target_model);
        edges.push({
          id: `e:${vmId(vm.name)}->${pid}#${idx}`,
          source: vmId(vm.name),
          sourceHandle,
          target: pid,
          type: "weighted",
          data: {
            weight: route.weight,
            weightPct: pct,
          } satisfies RouteEdgeData,
        });
      } else if (route.target_type === "virtual" && vmIndex.has(route.target_model)) {
        edges.push({
          id: `e:${vmId(vm.name)}->${vmId(route.target_model)}#${idx}`,
          source: vmId(vm.name),
          sourceHandle,
          target: vmId(route.target_model),
          type: "weighted",
          data: {
            weight: route.weight,
            weightPct: pct,
          } satisfies RouteEdgeData,
        });
      }
    });
  }

  return { nodes, edges };
}
