// Client-side dry-run. Resolves ${...} placeholders against sample parameter
// values so the user can see what each step/primitive would receive. This does
// NOT execute anything in a sandbox — step outputs are shown as symbolic tokens.

import { PLACEHOLDER_RE } from "./definition";
import type { ComposedDefinition, PrimitiveDefinition, ToolDefinition } from "./types";

export interface ResolvedStep {
  id: string;
  ref: string;
  inputs: Record<string, unknown>;
}

export interface SimulationResult {
  /** For primitive tools: the resolved args sent to the base primitive. */
  args?: Record<string, unknown>;
  /** For composed tools: the resolved inputs per step. */
  steps?: ResolvedStep[];
  /** For mock primitive tools: a human note (no resolution needed). */
  note?: string;
}

function resolveString(input: string, sample: Record<string, string>): string {
  return input.replace(PLACEHOLDER_RE, (_full, rawExpr: string) => {
    const expr = rawExpr.trim();
    if (expr === "parameters") return JSON.stringify(sample);
    if (expr.startsWith("params.")) {
      const name = expr.slice("params.".length);
      return sample[name] ?? `⟨${name}⟩`;
    }
    if (expr.startsWith("secrets.")) return `⟨secret:${expr.slice("secrets.".length)}⟩`;
    if (expr.startsWith("steps.")) return `⟨${expr}⟩`;
    // bare ${param} (primitive delegate syntax)
    return sample[expr] ?? `⟨${expr}⟩`;
  });
}

function resolveValue(value: unknown, sample: Record<string, string>): unknown {
  if (typeof value === "string") return resolveString(value, sample);
  if (Array.isArray(value)) return value.map((v) => resolveValue(v, sample));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([k, v]) => [k, resolveValue(v, sample)]),
    );
  }
  return value;
}

function resolveRecord(
  rec: Record<string, unknown> | undefined,
  sample: Record<string, string>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(rec ?? {})) {
    out[k] = resolveValue(v, sample);
  }
  return out;
}

export function simulate(
  def: ToolDefinition,
  sample: Record<string, string>,
): SimulationResult {
  if (def.tool_type === "primitive") return simulatePrimitive(def, sample);
  return simulateComposed(def, sample);
}

function simulatePrimitive(
  def: PrimitiveDefinition,
  sample: Record<string, string>,
): SimulationResult {
  if (def.implementation.mode === "mock") {
    const strategy = def.implementation.mock?.strategy ?? "static";
    return { note: `Mock tool (${strategy}) — returns a canned response without calling out.` };
  }
  return { args: resolveRecord(def.implementation.args, sample) };
}

function simulateComposed(
  def: ComposedDefinition,
  sample: Record<string, string>,
): SimulationResult {
  return {
    steps: def.steps.map((step) => ({
      id: step.id,
      ref: `${step.ref.type}:${step.ref.name}`,
      inputs: resolveRecord(step.inputs, sample),
    })),
  };
}
