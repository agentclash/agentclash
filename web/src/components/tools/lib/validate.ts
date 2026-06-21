// Client-side mirror of backend/internal/toolspec ValidateDefinition. This gives
// instant inline feedback in the builder; the server remains authoritative on save.

import { placeholdersIn } from "./definition";
import type {
  ComposedDefinition,
  MockConfig,
  PrimitiveDefinition,
  ToolDefinition,
  ToolPrimitive,
  ValidationIssue,
} from "./types";

const MOCK_STRATEGIES = new Set(["static", "lookup", "echo"]);

export interface ValidateContext {
  /** Delegatable base primitives, keyed by name (carries each one's schema). */
  primitives: Map<string, ToolPrimitive>;
  /** Slugs of other saved tools in the workspace (for composed tool refs). */
  knownToolSlugs?: Set<string>;
  /** Slug of the tool being edited (to reject self-reference). */
  selfSlug?: string;
}

export function validateDefinition(
  def: ToolDefinition,
  ctx: ValidateContext,
): ValidationIssue[] {
  if (def.tool_type === "primitive") return validatePrimitive(def, ctx);
  if (def.tool_type === "composed") return validateComposed(def, ctx);
  return [{ path: "tool_type", message: "must be primitive or composed" }];
}

function declared(def: ToolDefinition): Set<string> {
  return new Set(Object.keys(def.parameters?.properties ?? {}));
}

function validatePrimitive(
  def: PrimitiveDefinition,
  ctx: ValidateContext,
): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const impl = def.implementation;
  const params = declared(def);

  if (impl.mode === "delegate") {
    const primitive = (impl.primitive ?? "").trim();
    const spec = primitive ? ctx.primitives.get(primitive) : undefined;
    if (!primitive) {
      issues.push({ path: "implementation.primitive", message: "Choose a primitive to delegate to." });
    } else if (!spec) {
      issues.push({ path: "implementation.primitive", message: `Unknown or non-delegatable primitive "${primitive}".` });
    }
    const isHTTP = primitive === "http_request";
    for (const ph of placeholdersIn(impl.args ?? {})) {
      issues.push(...checkArgPlaceholder(ph, params, isHTTP));
    }
    if (spec) issues.push(...checkPrimitiveArgs("implementation.args", spec, impl.args));
  } else if (impl.mode === "mock") {
    issues.push(...checkMock(impl.mock));
  } else {
    issues.push({ path: "implementation.mode", message: "Mode must be delegate or mock." });
  }

  return issues;
}

/** Required args must be present (non-blank); unknown args are rejected. */
function checkPrimitiveArgs(
  path: string,
  spec: ToolPrimitive,
  args: Record<string, unknown> | undefined,
): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const props = spec.parameters?.properties ?? {};
  const required = spec.parameters?.required ?? [];
  const a = args ?? {};
  for (const r of required) {
    const v = a[r];
    if (v === undefined || v === null || (typeof v === "string" && v.trim() === "")) {
      issues.push({ path, message: `Missing required argument "${r}" for ${spec.name}.` });
    }
  }
  for (const k of Object.keys(a)) {
    if (!(k in props)) {
      issues.push({ path, message: `Unknown argument "${k}" for ${spec.name}.` });
    }
  }
  return issues;
}

/** Mirror engine.newMockTool per-strategy requirements. */
function checkMock(mock: MockConfig | undefined): ValidationIssue[] {
  if (!mock) return [{ path: "implementation.mock", message: "Configure the mock response." }];
  if (!MOCK_STRATEGIES.has(mock.strategy)) {
    return [{ path: "implementation.mock.strategy", message: "Strategy must be static, lookup or echo." }];
  }
  const issues: ValidationIssue[] = [];
  if (mock.strategy === "static" && mock.response === undefined) {
    issues.push({ path: "implementation.mock.response", message: 'A response is required for the "static" strategy.' });
  }
  if (mock.strategy === "lookup") {
    if (!mock.lookup_key?.trim()) {
      issues.push({ path: "implementation.mock.lookup_key", message: 'A lookup key is required for the "lookup" strategy.' });
    }
    if (mock.responses === undefined) {
      issues.push({ path: "implementation.mock.responses", message: 'Responses are required for the "lookup" strategy.' });
    }
  }
  if (mock.strategy === "echo" && mock.template === undefined) {
    issues.push({ path: "implementation.mock.template", message: 'A template is required for the "echo" strategy.' });
  }
  return issues;
}

function checkArgPlaceholder(ph: string, params: Set<string>, isHTTP: boolean): ValidationIssue[] {
  const expr = unwrapTemplateEncoding(ph.trim());
  if (expr === "") return [{ path: "implementation.args", message: "Empty placeholder ${}." }];
  if (expr === "parameters") return [];
  if (expr.startsWith("secrets.")) {
    return isHTTP ? [] : [{ path: "implementation.args", message: "Secrets are only allowed for http_request." }];
  }
  if (!params.has(expr)) {
    return [{ path: "implementation.args", message: `Placeholder \${${expr}} is not a declared parameter.` }];
  }
  return [];
}

function unwrapTemplateEncoding(expr: string): string {
  for (const encoding of ["json", "query", "path"]) {
    if (expr.startsWith(`${encoding}:`)) return expr.slice(encoding.length + 1);
  }
  return expr;
}

function validateComposed(
  def: ComposedDefinition,
  ctx: ValidateContext,
): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const params = declared(def);

  if (!def.steps || def.steps.length === 0) {
    return [{ path: "steps", message: "Add at least one step." }];
  }

  const seen = new Set<string>();
  def.steps.forEach((step, i) => {
    const base = `steps[${i}]`;
    const id = step.id?.trim();
    if (!id) {
      issues.push({ path: `${base}.id`, message: "Step id is required." });
    } else if (seen.has(id)) {
      issues.push({ path: `${base}.id`, message: `Duplicate step id "${id}".` });
    }

    const refName = step.ref?.name?.trim() ?? "";
    let isHTTP = false;
    if (step.ref?.type === "primitive") {
      const spec = ctx.primitives.get(refName);
      if (!spec) {
        issues.push({ path: `${base}.ref.name`, message: `Unknown primitive "${refName}".` });
      } else {
        issues.push(...checkPrimitiveArgs(`${base}.inputs`, spec, step.inputs));
      }
      isHTTP = refName === "http_request";
    } else if (step.ref?.type === "tool") {
      if (!refName) {
        issues.push({ path: `${base}.ref.name`, message: "Choose a tool." });
      }
      if (ctx.selfSlug && refName === ctx.selfSlug) {
        issues.push({ path: `${base}.ref.name`, message: "A tool cannot reference itself." });
      }
      if (ctx.knownToolSlugs && refName && !ctx.knownToolSlugs.has(refName)) {
        issues.push({ path: `${base}.ref.name`, message: `Unknown tool "${refName}".` });
      }
    } else {
      issues.push({ path: `${base}.ref.type`, message: "Reference must be a primitive or a tool." });
    }

    for (const ph of placeholdersIn(step.inputs ?? {})) {
      issues.push(...checkStepPlaceholder(ph, base, params, seen, isHTTP));
    }

    if (id) seen.add(id);
  });

  return issues;
}

function checkStepPlaceholder(
  ph: string,
  base: string,
  params: Set<string>,
  priorStepIds: Set<string>,
  isHTTP: boolean,
): ValidationIssue[] {
  const expr = ph.trim();
  if (expr === "") return [{ path: `${base}.inputs`, message: "Empty placeholder ${}." }];
  if (expr.startsWith("params.")) {
    const name = expr.slice("params.".length);
    return params.has(name) ? [] : [{ path: `${base}.inputs`, message: `Unknown parameter "${name}".` }];
  }
  if (expr.startsWith("steps.")) {
    const rest = expr.slice("steps.".length);
    const sid = rest.includes(".") ? rest.slice(0, rest.indexOf(".")) : rest;
    return priorStepIds.has(sid)
      ? []
      : [{ path: `${base}.inputs`, message: `Step "${sid}" is not an earlier step.` }];
  }
  if (expr.startsWith("secrets.")) {
    return isHTTP ? [] : [{ path: `${base}.inputs`, message: "Secrets are only allowed in http_request steps." }];
  }
  return [{ path: `${base}.inputs`, message: `Use \${params.X}, \${steps.ID.field} or \${secrets.NAME} (got \${${expr}}).` }];
}
