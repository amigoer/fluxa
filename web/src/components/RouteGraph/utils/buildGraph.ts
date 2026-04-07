// buildGraph.ts — converts a snapshot of admin API data into the
// (nodes, edges) shape that React Flow consumes.
//
// The graph is a single connected DAG with this shape:
//
//   SourceNode ─┬─► RegexRouteNode ─► VirtualModelNode ─┬─► ProviderNode
//               │                  └► ProviderNode      ├─► ProviderNode
//               │                                       └─► ProviderNode
//               ├─► VirtualModelNode (direct name match)
//               └─► FallbackNode (always last, dashed)
//
// Two important semantic rules embedded here:
//   1. A VirtualModel that no regex route points to is still a valid
//      runtime entry point (a request whose model name matches the
//      VM's name resolves directly). We draw a source→VM edge for
//      every such VM so the graph is *always* connected — never a
//      floating subgraph.
//   2. Disabled regex routes and disabled VM routes are skipped.
//      Disabled rules do not run at the data plane, so showing them
//      on the topology view would be misleading.
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
  labelKind?: "priority" | "matched" | "noMatch" | "direct";
  priority?: number;
}

// SEGMENT_PALETTE is the colour ring used for VirtualModelNode weight
// bar segments. We deliberately stick to four shades of one purple
// family rather than a multi-hue palette: the user reads the bar as
// "this VM splits into N pieces", and rainbow segments would imply
// the targets are categorically different when in fact they are all
// downstream of the same alias.
const SEGMENT_PALETTE = [
  "#534AB7", // deepest
  "#7F77DD",
  "#AFA9EC",
  "#CECBF6", // lightest
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
  //    without a second pass over the list. Disabled VM routes are
  //    stripped here so the segment colour palette stays in lockstep
  //    with the actually-rendered fanout edges below.
  const vmIndex = new Map<string, VirtualModel>();
  for (const vm of virtualModels) {
    const enabledRoutes = vm.routes.filter((r) => r.enabled !== false);
    const visibleVm: VirtualModel = { ...vm, routes: enabledRoutes };
    vmIndex.set(vm.name, visibleVm);
    const colors = enabledRoutes.map(
      (_, i) => SEGMENT_PALETTE[i % SEGMENT_PALETTE.length],
    );
    nodes.push({
      id: vmId(vm.name),
      type: "virtualModel",
      position: { x: 0, y: 0 },
      data: { model: visibleVm, colors } satisfies VirtualModelNodeData,
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
  //    highest priority). Disabled rules are filtered out because the
  //    data plane skips them at runtime; showing them on the topology
  //    view would imply they take traffic. The visual stacking matches
  //    the runtime evaluation order, which is the single most
  //    important thing the operator wants to verify on this screen.
  //
  //    Side effect: we record which VMs are reached via a regex match
  //    so the next loop knows whether to draw a "direct name match"
  //    fallback edge into them.
  const regexTargetedVMs = new Set<string>();
  const sortedRegex = [...regexRoutes]
    .filter((r) => r.enabled !== false)
    .sort((a, b) => (a.priority ?? 100) - (b.priority ?? 100));
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
      regexTargetedVMs.add(r.target_model);
      edges.push({
        id: `e:${id}->${vmId(r.target_model)}`,
        source: id,
        target: vmId(r.target_model),
        type: "route",
        data: { labelKind: "matched" } satisfies RouteEdgeData,
      });
    }
  }

  // 4b. Direct source→VM edges. Any virtual model that no enabled
  //     regex route points at is still reachable at runtime by sending
  //     the request with model=<vm name>. Without these edges, such a
  //     VM would float orphaned on the canvas and the operator could
  //     not see how requests reach it. The edge is the same "route"
  //     type as source→regex (solid gray) and carries no label —
  //     "direct" mode is the implicit default for virtual models.
  for (const vm of virtualModels) {
    if (regexTargetedVMs.has(vm.name)) continue;
    edges.push({
      id: `e:${SOURCE_ID}->${vmId(vm.name)}`,
      source: SOURCE_ID,
      target: vmId(vm.name),
      type: "route",
      data: { labelKind: "direct" } satisfies RouteEdgeData,
    });
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

  // 6. Virtual model fanout edges. We iterate the *visible* (enabled-
  //    only) routes from vmIndex so the edges and the segment colours
  //    in the VM card stay perfectly aligned by index. Each enabled
  //    route gets its own outgoing edge with its own source handle —
  //    without per-route handles dagre routes every fanout line
  //    through one point and the layout looks crowded.
  //
  //    The percentage shown on the edge is computed by normalising
  //    against the sum of *visible* weights. When the operator has
  //    typed weights that already sum to 100 this collapses to the
  //    raw integer; when they don't, we still show an honest split.
  for (const vm of virtualModels) {
    const visibleVm = vmIndex.get(vm.name);
    if (!visibleVm) continue;
    const total = visibleVm.routes.reduce(
      (acc: number, r: VirtualModelRoute) => acc + (r.weight || 0),
      0,
    );
    visibleVm.routes.forEach((route, idx) => {
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
