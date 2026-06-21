import { describe, expect, it } from "vitest";

import {
  compileGraph,
  inferKind,
  INPUTS_NODE_ID,
  nodeReferences,
  parseDefinition,
  validateGraph,
  type CanvasNode,
  type ToolGraph,
} from "./graph";
import type { ComposedDefinition, PrimitiveDefinition } from "./types";

const schema = {
  type: "object" as const,
  properties: { order_id: { type: "string" as const } },
  required: ["order_id"],
  additionalProperties: false,
};

function inputs(): CanvasNode {
  return { id: INPUTS_NODE_ID, kind: "inputs", position: { x: 0, y: 0 }, data: { parameters: schema } };
}

describe("inferKind", () => {
  it("single operation → primitive", () => {
    expect(inferKind([inputs(), { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: {} }])).toBe(
      "primitive",
    );
  });
  it("canned alone → primitive", () => {
    expect(inferKind([inputs(), { id: "c", kind: "canned", position: { x: 0, y: 0 }, data: {} }])).toBe(
      "primitive",
    );
  });
  it("two operations → composed", () => {
    expect(
      inferKind([
        inputs(),
        { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: {} },
        { id: "s2", kind: "operation", position: { x: 0, y: 0 }, data: {} },
      ]),
    ).toBe("composed");
  });
  it("single tool reference → composed", () => {
    expect(inferKind([inputs(), { id: "s1", kind: "tool", position: { x: 0, y: 0 }, data: {} }])).toBe(
      "composed",
    );
  });
});

describe("compileGraph primitive", () => {
  it("delegate translates ${params.x} → ${x}", () => {
    const graph: ToolGraph = {
      nodes: [
        inputs(),
        {
          id: "s1",
          kind: "operation",
          position: { x: 0, y: 0 },
          data: {
            stepId: "s1",
            primitive: "http_request",
            inputs: { url: "https://api/${params.order_id}" },
          },
        },
      ],
      edges: [{ id: "e", source: INPUTS_NODE_ID, target: "s1" }],
    };
    const def = compileGraph(graph, "primitive") as PrimitiveDefinition;
    expect(def.tool_type).toBe("primitive");
    expect(def.implementation.mode).toBe("delegate");
    expect(def.implementation.primitive).toBe("http_request");
    expect(def.implementation.args).toEqual({ url: "https://api/${order_id}" });
  });

  it("canned node → mock primitive", () => {
    const graph: ToolGraph = {
      nodes: [inputs(), { id: "c", kind: "canned", position: { x: 0, y: 0 }, data: { mock: { strategy: "static", response: { ok: true } } } }],
      edges: [],
    };
    const def = compileGraph(graph, "primitive") as PrimitiveDefinition;
    expect(def.implementation.mode).toBe("mock");
    expect(def.implementation.mock).toEqual({ strategy: "static", response: { ok: true } });
  });
});

describe("compileGraph composed", () => {
  it("orders steps by edges and keeps composed syntax", () => {
    const graph: ToolGraph = {
      nodes: [
        inputs(),
        { id: "b", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s2", primitive: "file_write", inputs: { content: "${steps.s1.output}" } } },
        { id: "a", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s1", primitive: "http_request", inputs: { url: "${params.order_id}" } } },
      ],
      // a (s1) must come before b (s2)
      edges: [{ id: "e", source: "a", target: "b" }],
    };
    const def = compileGraph(graph, "composed") as ComposedDefinition;
    expect(def.tool_type).toBe("composed");
    expect(def.steps.map((s) => s.id)).toEqual(["s1", "s2"]);
    expect(def.steps[1].inputs).toEqual({ content: "${steps.s1.output}" });
  });
});

describe("parseDefinition", () => {
  it("primitive delegate → one op node + inputs edge, args back to ${params.x}", () => {
    const def: PrimitiveDefinition = {
      tool_type: "primitive",
      description: "",
      parameters: schema,
      implementation: { mode: "delegate", primitive: "http_request", args: { url: "https://api/${order_id}" } },
    };
    const g = parseDefinition(def);
    const op = g.nodes.find((n) => n.kind === "operation")!;
    expect(op.data.primitive).toBe("http_request");
    expect(op.data.inputs).toEqual({ url: "https://api/${params.order_id}" });
    expect(g.edges.some((e) => e.source === INPUTS_NODE_ID && e.target === op.id)).toBe(true);
  });

  it("composed → nodes + edges inferred from references", () => {
    const def: ComposedDefinition = {
      tool_type: "composed",
      description: "",
      parameters: schema,
      steps: [
        { id: "s1", ref: { type: "primitive", name: "http_request" }, inputs: { url: "${params.order_id}" } },
        { id: "s2", ref: { type: "primitive", name: "file_write" }, inputs: { content: "${steps.s1.output}" } },
      ],
    };
    const g = parseDefinition(def);
    expect(g.nodes.filter((n) => n.kind === "operation")).toHaveLength(2);
    expect(g.edges.some((e) => e.source === "s1" && e.target === "s2")).toBe(true);
    expect(g.edges.some((e) => e.source === INPUTS_NODE_ID && e.target === "s1")).toBe(true);
  });

  it("round-trips a composed definition", () => {
    const def: ComposedDefinition = {
      tool_type: "composed",
      description: "do things",
      parameters: schema,
      steps: [
        { id: "s1", ref: { type: "primitive", name: "http_request" }, inputs: { url: "${params.order_id}" } },
        { id: "s2", ref: { type: "tool", name: "save_it" }, inputs: { content: "${steps.s1.output}" } },
      ],
    };
    const recompiled = compileGraph(parseDefinition(def), "composed", { description: "do things" });
    expect(recompiled).toEqual(def);
  });
});

describe("nodeReferences", () => {
  it("offers agent inputs from a wired Inputs node and results from upstream steps", () => {
    const graph: ToolGraph = {
      nodes: [
        inputs(),
        { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s1", primitive: "http_request" } },
        { id: "s2", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s2", primitive: "file_write" } },
      ],
      edges: [
        { id: "e1", source: INPUTS_NODE_ID, target: "s2" },
        { id: "e2", source: "s1", target: "s2" },
      ],
    };
    const refs = nodeReferences("s2", graph, (n) => n.data.primitive ?? "op");
    expect(refs).toEqual(
      expect.arrayContaining([
        { label: "order_id", group: "Agent inputs", token: "${params.order_id}" },
        { label: "Result of http_request", group: "Earlier steps", token: "${steps.s1.output}" },
      ]),
    );
  });

  it("offers nothing to a node with no incoming wires", () => {
    const graph: ToolGraph = {
      nodes: [inputs(), { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s1" } }],
      edges: [],
    };
    expect(nodeReferences("s1", graph, () => "x")).toEqual([]);
  });
});

describe("validateGraph", () => {
  it("flags canned combined with steps", () => {
    const nodes: CanvasNode[] = [
      inputs(),
      { id: "c", kind: "canned", position: { x: 0, y: 0 }, data: {} },
      { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: {} },
    ];
    const issues = validateGraph({ nodes, edges: [] }, inferKind(nodes));
    expect(issues.some((i) => /canned/i.test(i.message))).toBe(true);
  });

  it("flags an empty graph", () => {
    const issues = validateGraph({ nodes: [inputs()], edges: [] }, "primitive");
    expect(issues.some((i) => /at least one/i.test(i.message))).toBe(true);
  });

  it("flags a cycle between steps so a corrupt order can't be saved", () => {
    const nodes: CanvasNode[] = [
      inputs(),
      { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s1", primitive: "http_request" } },
      { id: "s2", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s2", primitive: "file_write" } },
    ];
    const edges = [
      { id: "e1", source: "s1", target: "s2" },
      { id: "e2", source: "s2", target: "s1" }, // back-edge → cycle
    ];
    const issues = validateGraph({ nodes, edges }, inferKind(nodes));
    expect(issues.some((i) => /loop/i.test(i.message))).toBe(true);
  });

  it("does not flag a valid linear chain as a cycle", () => {
    const nodes: CanvasNode[] = [
      inputs(),
      { id: "s1", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s1", primitive: "http_request" } },
      { id: "s2", kind: "operation", position: { x: 0, y: 0 }, data: { stepId: "s2", primitive: "file_write" } },
    ];
    const edges = [
      { id: "e1", source: INPUTS_NODE_ID, target: "s1" },
      { id: "e2", source: "s1", target: "s2" },
    ];
    const issues = validateGraph({ nodes, edges }, inferKind(nodes));
    expect(issues.some((i) => /loop/i.test(i.message))).toBe(false);
  });
});
