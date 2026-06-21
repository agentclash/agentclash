// Pure, framework-agnostic translation between the visual node canvas and the
// persisted tool `definition`. No React, no React Flow — fully unit-testable.
//
// The canvas hides the primitive/composed split: one operation node compiles to
// a `primitive` tool, a canned-response node compiles to a `primitive` mock, and
// anything wired into a chain compiles to a `composed` tool. Internally every
// value reference is stored in composed syntax (`${params.x}`, `${steps.id.output}`)
// and translated to bare `${x}` only when emitting a primitive definition.

import { PLACEHOLDER_RE } from "./definition";
import type { ValueReference } from "./friendly";
import type {
  ComposedDefinition,
  ComposedStep,
  JsonSchemaObject,
  MockConfig,
  PrimitiveDefinition,
  ToolDefinition,
  ToolType,
} from "./types";

export type CanvasNodeKind = "inputs" | "operation" | "tool" | "canned";

// Extends Record so it satisfies React Flow's node-data constraint directly.
export interface CanvasNodeData extends Record<string, unknown> {
  /** Stable step id (`s1`, `s2`, …) for operation/tool nodes. */
  stepId?: string;
  /** operation: the base primitive name. */
  primitive?: string;
  /** tool: the referenced saved-tool slug + display name. */
  slug?: string;
  toolName?: string;
  /** operation/tool: input map in composed syntax. */
  inputs?: Record<string, unknown>;
  /** canned: the mock response config. */
  mock?: MockConfig;
  /** inputs node: the tool's parameter schema (what the agent provides). */
  parameters?: JsonSchemaObject;
}

export interface CanvasNode {
  id: string;
  kind: CanvasNodeKind;
  position: { x: number; y: number };
  data: CanvasNodeData;
}

export interface CanvasEdge {
  id: string;
  source: string;
  target: string;
}

export interface ToolGraph {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
}

export const INPUTS_NODE_ID = "inputs";

// --- small helpers -----------------------------------------------------------

function stepNodes(nodes: CanvasNode[]): CanvasNode[] {
  return nodes.filter((n) => n.kind === "operation" || n.kind === "tool");
}

export function inputsNode(nodes: CanvasNode[]): CanvasNode | undefined {
  return nodes.find((n) => n.kind === "inputs");
}

export function nextStepId(nodes: CanvasNode[]): string {
  const used = new Set(nodes.map((n) => n.data.stepId).filter(Boolean));
  let i = 1;
  while (used.has(`s${i}`)) i += 1;
  return `s${i}`;
}

/** Map every string in a value tree through `fn`. */
function mapStrings(value: unknown, fn: (s: string) => string): unknown {
  if (typeof value === "string") return fn(value);
  if (Array.isArray(value)) return value.map((v) => mapStrings(v, fn));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([k, v]) => [k, mapStrings(v, fn)]),
    );
  }
  return value;
}

function mapInputs(
  inputs: Record<string, unknown> | undefined,
  fn: (s: string) => string,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(inputs ?? {})) out[k] = mapStrings(v, fn);
  return out;
}

/** Composed/internal token syntax → primitive arg syntax (`${params.x}` → `${x}`). */
function toPrimitiveInputs(inputs: Record<string, unknown> | undefined): Record<string, unknown> {
  return mapInputs(inputs, (s) =>
    s.replace(PLACEHOLDER_RE, (full, raw: string) => {
      const expr = raw.trim();
      return expr.startsWith("params.") ? `\${${expr.slice("params.".length)}}` : full;
    }),
  );
}

/** Primitive arg syntax → internal composed syntax (`${x}` → `${params.x}`). */
function fromPrimitiveInputs(
  args: Record<string, unknown> | undefined,
  declared: Set<string>,
): Record<string, unknown> {
  return mapInputs(args, (s) =>
    s.replace(PLACEHOLDER_RE, (full, raw: string) => {
      const expr = raw.trim();
      if (expr === "parameters" || expr.startsWith("secrets.") || expr.startsWith("params.")) {
        return full;
      }
      return declared.has(expr) ? `\${params.${expr}}` : full;
    }),
  );
}

// --- kind inference ----------------------------------------------------------

/** The tool_kind a freshly-built graph should be saved as. */
export function inferKind(nodes: CanvasNode[]): ToolType {
  const ops = nodes.filter((n) => n.kind === "operation").length;
  const tools = nodes.filter((n) => n.kind === "tool").length;
  const canned = nodes.filter((n) => n.kind === "canned").length;
  if (canned > 0 && ops === 0 && tools === 0) return "primitive";
  if (ops <= 1 && tools === 0 && canned === 0) return "primitive";
  return "composed";
}

// --- ordering ----------------------------------------------------------------

/** Topologically order step nodes so dependencies precede dependents. */
function orderedSteps(nodes: CanvasNode[], edges: CanvasEdge[]): CanvasNode[] {
  const steps = stepNodes(nodes);
  const ids = new Set(steps.map((n) => n.id));
  const indeg = new Map<string, number>(steps.map((n) => [n.id, 0]));
  const outs = new Map<string, string[]>(steps.map((n) => [n.id, []]));
  for (const e of edges) {
    if (ids.has(e.source) && ids.has(e.target) && e.source !== e.target) {
      outs.get(e.source)!.push(e.target);
      indeg.set(e.target, (indeg.get(e.target) ?? 0) + 1);
    }
  }
  // Stable: process in original array order among ready nodes.
  const order = steps.map((n) => n.id);
  const ready = order.filter((id) => (indeg.get(id) ?? 0) === 0);
  const result: string[] = [];
  const seen = new Set<string>();
  while (ready.length) {
    const id = ready.shift()!;
    if (seen.has(id)) continue;
    seen.add(id);
    result.push(id);
    for (const t of outs.get(id) ?? []) {
      indeg.set(t, (indeg.get(t) ?? 1) - 1);
      if ((indeg.get(t) ?? 0) === 0) ready.push(t);
    }
    ready.sort((a, b) => order.indexOf(a) - order.indexOf(b));
  }
  // Append any nodes left out by a cycle, in original order. A cyclic graph is
  // blocked upstream by validateGraph (hasCycle), so this only runs defensively.
  for (const id of order) if (!seen.has(id)) result.push(id);
  const byId = new Map(steps.map((n) => [n.id, n]));
  return result.map((id) => byId.get(id)!).filter(Boolean);
}

/**
 * True if the step nodes contain a directed cycle (e.g. a back-edge A → B → A).
 * Such a graph can't be topologically ordered, so the compiled `composed`
 * definition would list steps out of dependency order — a step could reference
 * another whose result hasn't been computed yet. Uses the same edge filtering
 * as `orderedSteps` (ignores self-loops and edges to/from non-step nodes).
 */
function hasCycle(nodes: CanvasNode[], edges: CanvasEdge[]): boolean {
  const steps = stepNodes(nodes);
  const ids = new Set(steps.map((n) => n.id));
  const outs = new Map<string, string[]>(steps.map((n) => [n.id, []]));
  for (const e of edges) {
    if (ids.has(e.source) && ids.has(e.target) && e.source !== e.target) {
      outs.get(e.source)!.push(e.target);
    }
  }
  // 3-colour DFS: gray = on the current path, black = fully explored.
  const color = new Map<string, 0 | 1 | 2>(steps.map((n) => [n.id, 0]));
  const visit = (id: string): boolean => {
    color.set(id, 1);
    for (const t of outs.get(id) ?? []) {
      const c = color.get(t);
      if (c === 1) return true; // back-edge into the current path
      if (c === 0 && visit(t)) return true;
    }
    color.set(id, 2);
    return false;
  };
  for (const n of steps) {
    if (color.get(n.id) === 0 && visit(n.id)) return true;
  }
  return false;
}

// --- compile: graph → definition --------------------------------------------

export function compileGraph(
  graph: ToolGraph,
  kind: ToolType,
  opts: { description?: string } = {},
): ToolDefinition {
  const parameters = inputsNode(graph.nodes)?.data.parameters ?? emptySchema();
  const description = opts.description ?? "";

  if (kind === "primitive") {
    const canned = graph.nodes.find((n) => n.kind === "canned");
    if (canned) {
      const def: PrimitiveDefinition = {
        tool_type: "primitive",
        description,
        parameters,
        implementation: { mode: "mock", mock: canned.data.mock ?? { strategy: "static" } },
      };
      return def;
    }
    const op = graph.nodes.find((n) => n.kind === "operation");
    const def: PrimitiveDefinition = {
      tool_type: "primitive",
      description,
      parameters,
      implementation: {
        mode: "delegate",
        primitive: op?.data.primitive ?? "",
        args: toPrimitiveInputs(op?.data.inputs),
      },
    };
    return def;
  }

  const steps: ComposedStep[] = orderedSteps(graph.nodes, graph.edges).map((n) => ({
    id: n.data.stepId ?? n.id,
    ref:
      n.kind === "operation"
        ? { type: "primitive", name: n.data.primitive ?? "" }
        : { type: "tool", name: n.data.slug ?? "" },
    inputs: n.data.inputs ?? {},
  }));
  const def: ComposedDefinition = { tool_type: "composed", description, parameters, steps };
  return def;
}

function emptySchema(): JsonSchemaObject {
  return { type: "object", properties: {}, required: [], additionalProperties: false };
}

// --- graph-level validation (friendly, complements validateDefinition) -------

export interface GraphIssue {
  message: string;
}

export function validateGraph(graph: ToolGraph, kind: ToolType): GraphIssue[] {
  const issues: GraphIssue[] = [];
  const ops = graph.nodes.filter((n) => n.kind === "operation");
  const tools = graph.nodes.filter((n) => n.kind === "tool");
  const canned = graph.nodes.filter((n) => n.kind === "canned");

  if (ops.length + tools.length + canned.length === 0) {
    issues.push({ message: "Add at least one node — an operation, another tool, or a canned response." });
  }
  if (canned.length > 1) {
    issues.push({ message: "Only one canned-response node is allowed." });
  }
  if (canned.length > 0 && ops.length + tools.length > 0) {
    issues.push({
      message: "A canned response can't be combined with other steps. Remove it, or remove the other nodes.",
    });
  }
  if (kind === "primitive" && (ops.length + tools.length > 1 || tools.length > 0)) {
    issues.push({
      message:
        "This is a single-action tool, so it can't have multiple steps. Create a new tool to build a chain.",
    });
  }
  if (hasCycle(graph.nodes, graph.edges)) {
    issues.push({
      message: "Steps are connected in a loop. Remove the connection that points back to an earlier step.",
    });
  }
  return issues;
}

// --- parse: definition → graph (with a simple layered layout) ----------------

const COL = 280;
const ROW = 140;

export function parseDefinition(def: ToolDefinition): ToolGraph {
  const declared = new Set(Object.keys(def.parameters?.properties ?? {}));
  const nodes: CanvasNode[] = [
    {
      id: INPUTS_NODE_ID,
      kind: "inputs",
      position: { x: 0, y: 0 },
      data: { parameters: def.parameters ?? emptySchema() },
    },
  ];
  const edges: CanvasEdge[] = [];

  if (def.tool_type === "primitive") {
    if (def.implementation.mode === "mock") {
      nodes.push({
        id: "canned",
        kind: "canned",
        position: { x: COL, y: 0 },
        data: { mock: def.implementation.mock ?? { strategy: "static" } },
      });
      edges.push({ id: "e-inputs-canned", source: INPUTS_NODE_ID, target: "canned" });
    } else {
      nodes.push({
        id: "s1",
        kind: "operation",
        position: { x: COL, y: 0 },
        data: {
          stepId: "s1",
          primitive: def.implementation.primitive ?? "",
          inputs: fromPrimitiveInputs(def.implementation.args, declared),
        },
      });
      edges.push({ id: "e-inputs-s1", source: INPUTS_NODE_ID, target: "s1" });
    }
    return layout({ nodes, edges });
  }

  // composed: one node per step, edges inferred from placeholder references.
  const stepIdToNode = new Map<string, string>();
  def.steps.forEach((step, i) => {
    const nodeId = step.id || `s${i + 1}`;
    stepIdToNode.set(step.id, nodeId);
    nodes.push({
      id: nodeId,
      kind: step.ref.type === "tool" ? "tool" : "operation",
      position: { x: COL, y: i * ROW },
      data:
        step.ref.type === "tool"
          ? { stepId: step.id, slug: step.ref.name, toolName: step.ref.name, inputs: step.inputs ?? {} }
          : { stepId: step.id, primitive: step.ref.name, inputs: step.inputs ?? {} },
    });
  });
  // Edges from references inside each step's inputs.
  def.steps.forEach((step) => {
    const target = stepIdToNode.get(step.id)!;
    const refs = referencesIn(step.inputs ?? {});
    if (refs.usesParams) {
      edges.push({ id: `e-inputs-${target}`, source: INPUTS_NODE_ID, target });
    }
    for (const fromStep of refs.stepIds) {
      const source = stepIdToNode.get(fromStep);
      if (source) edges.push({ id: `e-${source}-${target}`, source, target });
    }
  });
  return layout({ nodes, edges });
}

function referencesIn(inputs: Record<string, unknown>): { usesParams: boolean; stepIds: string[] } {
  let usesParams = false;
  const stepIds = new Set<string>();
  mapStrings(inputs, (s) => {
    for (const m of s.matchAll(PLACEHOLDER_RE)) {
      const expr = m[1].trim();
      if (expr.startsWith("params.")) usesParams = true;
      else if (expr.startsWith("steps.")) {
        const rest = expr.slice("steps.".length);
        stepIds.add(rest.includes(".") ? rest.slice(0, rest.indexOf(".")) : rest);
      }
    }
    return s;
  });
  return { usesParams, stepIds: [...stepIds] };
}

/**
 * The values a node can insert, based on what is wired into it: agent inputs
 * (when the Inputs node feeds it) and the result of each upstream step.
 */
export function nodeReferences(
  targetId: string,
  graph: ToolGraph,
  labelFor: (node: CanvasNode) => string,
): ValueReference[] {
  const byId = new Map(graph.nodes.map((n) => [n.id, n]));
  const sources = graph.edges.filter((e) => e.target === targetId).map((e) => byId.get(e.source));
  const refs: ValueReference[] = [];
  for (const src of sources) {
    if (!src) continue;
    if (src.kind === "inputs") {
      for (const name of Object.keys(src.data.parameters?.properties ?? {})) {
        refs.push({ label: name, group: "Agent inputs", token: `\${params.${name}}` });
      }
    } else if (src.data.stepId) {
      refs.push({
        label: `Result of ${labelFor(src)}`,
        group: "Earlier steps",
        token: `\${steps.${src.data.stepId}.output}`,
      });
    }
  }
  return refs;
}

/** Assign a simple left-to-right layered layout based on edge depth. */
export function layout(graph: ToolGraph): ToolGraph {
  const depth = new Map<string, number>();
  const incoming = new Map<string, string[]>();
  for (const n of graph.nodes) incoming.set(n.id, []);
  for (const e of graph.edges) incoming.get(e.target)?.push(e.source);

  const visiting = new Set<string>();
  const compute = (id: string): number => {
    if (depth.has(id)) return depth.get(id)!;
    if (visiting.has(id)) return 0; // cycle guard
    visiting.add(id);
    const ins = incoming.get(id) ?? [];
    const d = ins.length === 0 ? 0 : Math.max(...ins.map(compute)) + 1;
    visiting.delete(id);
    depth.set(id, d);
    return d;
  };
  for (const n of graph.nodes) compute(n.id);

  const rowByCol = new Map<number, number>();
  const nodes = graph.nodes.map((n) => {
    const col = n.kind === "inputs" ? 0 : (depth.get(n.id) ?? 1) || 1;
    const row = rowByCol.get(col) ?? 0;
    rowByCol.set(col, row + 1);
    return { ...n, position: { x: col * COL, y: row * ROW } };
  });
  return { ...graph, nodes };
}
