import { describe, expect, it } from "vitest";
import {
  emptyComposedDefinition,
  emptyPrimitiveDefinition,
  paramsToSchema,
  placeholdersIn,
  reorder,
  schemaToParams,
} from "./definition";
import { validateDefinition } from "./validate";
import { simulate } from "./simulate";
import type { ComposedDefinition, PrimitiveDefinition } from "./types";

const ctx = (over: Partial<Parameters<typeof validateDefinition>[1]> = {}) => ({
  delegatablePrimitives: new Set(["http_request", "read_file"]),
  knownToolSlugs: new Set(["check_policy"]),
  ...over,
});

describe("definition helpers", () => {
  it("round-trips params <-> schema", () => {
    const params = [
      { name: "order_id", type: "string" as const, required: true, description: "the id" },
      { name: "amount", type: "number" as const, required: false },
    ];
    const schema = paramsToSchema(params);
    expect(schema.properties.order_id).toEqual({ type: "string", description: "the id" });
    expect(schema.required).toEqual(["order_id"]);
    const back = schemaToParams(schema);
    expect(back).toHaveLength(2);
    expect(back[0]).toMatchObject({ name: "order_id", required: true });
  });

  it("drops nameless params", () => {
    const schema = paramsToSchema([{ name: "  ", type: "string", required: true }]);
    expect(Object.keys(schema.properties)).toHaveLength(0);
  });

  it("reorder moves items immutably", () => {
    const a = [1, 2, 3];
    const b = reorder(a, 0, 2);
    expect(b).toEqual([2, 3, 1]);
    expect(a).toEqual([1, 2, 3]);
    expect(reorder(a, 0, 9)).toBe(a);
  });

  it("extracts placeholders from nested structures", () => {
    const phs = placeholdersIn({ url: "https://x/${order_id}", h: { a: "${secrets.K}" }, list: ["${amount}"] });
    expect(phs.sort()).toEqual(["amount", "order_id", "secrets.K"]);
  });
});

describe("validateDefinition - primitive", () => {
  it("passes a valid http delegate", () => {
    const def = emptyPrimitiveDefinition();
    def.parameters = paramsToSchema([{ name: "order_id", type: "string", required: true }]);
    def.implementation = { mode: "delegate", primitive: "http_request", args: { url: "https://x/${order_id}", auth: "${secrets.K}" } };
    expect(validateDefinition(def, ctx())).toEqual([]);
  });

  it("flags unknown primitive", () => {
    const def = emptyPrimitiveDefinition();
    def.implementation = { mode: "delegate", primitive: "teleport", args: {} };
    expect(validateDefinition(def, ctx()).some((i) => i.path === "implementation.primitive")).toBe(true);
  });

  it("flags undeclared placeholder", () => {
    const def = emptyPrimitiveDefinition();
    def.implementation = { mode: "delegate", primitive: "http_request", args: { url: "${missing}" } };
    expect(validateDefinition(def, ctx()).some((i) => i.path === "implementation.args")).toBe(true);
  });

  it("rejects secrets for non-http", () => {
    const def = emptyPrimitiveDefinition();
    def.parameters = paramsToSchema([{ name: "p", type: "string", required: false }]);
    def.implementation = { mode: "delegate", primitive: "read_file", args: { path: "${secrets.X}" } };
    expect(validateDefinition(def, ctx()).some((i) => i.path === "implementation.args")).toBe(true);
  });

  it("validates mock strategy", () => {
    const def = emptyPrimitiveDefinition();
    def.implementation = { mode: "mock", mock: { strategy: "weird" as never } };
    expect(validateDefinition(def, ctx()).some((i) => i.path === "implementation.mock.strategy")).toBe(true);
  });
});

describe("validateDefinition - composed", () => {
  const base = (): ComposedDefinition => {
    const d = emptyComposedDefinition();
    d.parameters = paramsToSchema([
      { name: "order_id", type: "string", required: true },
      { name: "amount", type: "number", required: false },
    ]);
    return d;
  };

  it("passes a valid two-step chain", () => {
    const d = base();
    d.steps = [
      { id: "s1", ref: { type: "primitive", name: "http_request" }, inputs: { url: "https://x/${params.order_id}" } },
      { id: "s2", ref: { type: "tool", name: "check_policy" }, inputs: { total: "${params.amount}", prior: "${steps.s1.body}" } },
    ];
    expect(validateDefinition(d, ctx({ selfSlug: "refund_flow" }))).toEqual([]);
  });

  it("flags empty steps", () => {
    expect(validateDefinition(base(), ctx())[0].path).toBe("steps");
  });

  it("flags forward step reference", () => {
    const d = base();
    d.steps = [
      { id: "s1", ref: { type: "primitive", name: "http_request" }, inputs: { url: "${steps.s2.x}" } },
      { id: "s2", ref: { type: "primitive", name: "http_request" }, inputs: { url: "https://x" } },
    ];
    expect(validateDefinition(d, ctx()).some((i) => i.path === "steps[0].inputs")).toBe(true);
  });

  it("flags self reference and unknown tool", () => {
    const d = base();
    d.steps = [{ id: "s1", ref: { type: "tool", name: "refund_flow" }, inputs: {} }];
    const issues = validateDefinition(d, ctx({ selfSlug: "refund_flow", knownToolSlugs: new Set(["refund_flow"]) }));
    expect(issues.some((i) => i.message.includes("itself"))).toBe(true);
  });
});

describe("simulate", () => {
  it("resolves primitive args from sample", () => {
    const def: PrimitiveDefinition = emptyPrimitiveDefinition();
    def.implementation = { mode: "delegate", primitive: "http_request", args: { url: "https://x/${order_id}", k: "${secrets.K}" } };
    const res = simulate(def, { order_id: "42" });
    expect(res.args?.url).toBe("https://x/42");
    expect(res.args?.k).toContain("secret:K");
  });

  it("notes mock tools", () => {
    const def = emptyPrimitiveDefinition();
    def.implementation = { mode: "mock", mock: { strategy: "static", response: {} } };
    expect(simulate(def, {}).note).toContain("Mock");
  });

  it("resolves composed step inputs and shows step outputs symbolically", () => {
    const def = emptyComposedDefinition();
    def.steps = [
      { id: "s1", ref: { type: "primitive", name: "http_request" }, inputs: { url: "https://x/${params.order_id}" } },
      { id: "s2", ref: { type: "tool", name: "check_policy" }, inputs: { prior: "${steps.s1.body}" } },
    ];
    const res = simulate(def, { order_id: "7" });
    expect(res.steps?.[0].inputs.url).toBe("https://x/7");
    expect(res.steps?.[1].inputs.prior).toBe("⟨steps.s1.body⟩");
  });
});
