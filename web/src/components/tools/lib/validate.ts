// Client-side mirror of backend/internal/toolspec ValidateDefinition. This gives
// instant inline feedback in the builder; the server remains authoritative on save.

import { placeholdersIn } from "./definition";
import type {
  ComposedDefinition,
  PrimitiveDefinition,
  ToolDefinition,
  ValidationIssue,
} from "./types";

const MOCK_STRATEGIES = new Set(["static", "lookup", "echo"]);

export interface ValidateContext {
  /** Names of base primitives that may be delegated to / composed. */
  delegatablePrimitives: Set<string>;
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
    if (!primitive) {
      issues.push({ path: "implementation.primitive", message: "Choose a primitive to delegate to." });
    } else if (!ctx.delegatablePrimitives.has(primitive)) {
      issues.push({ path: "implementation.primitive", message: `Unknown or non-delegatable primitive "${primitive}".` });
    }
    const isHTTP = primitive === "http_request";
    for (const ph of placeholdersIn(impl.args ?? {})) {
      issues.push(...checkArgPlaceholder(ph, params, isHTTP));
    }
  } else if (impl.mode === "mock") {
    if (!impl.mock) {
      issues.push({ path: "implementation.mock", message: "Configure the mock response." });
    } else if (!MOCK_STRATEGIES.has(impl.mock.strategy)) {
      issues.push({ path: "implementation.mock.strategy", message: "Strategy must be static, lookup or echo." });
    }
  } else {
    issues.push({ path: "implementation.mode", message: "Mode must be delegate or mock." });
  }

  return issues;
}

function checkArgPlaceholder(ph: string, params: Set<string>, isHTTP: boolean): ValidationIssue[] {
  const expr = ph.trim();
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
      if (!ctx.delegatablePrimitives.has(refName)) {
        issues.push({ path: `${base}.ref.name`, message: `Unknown primitive "${refName}".` });
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
