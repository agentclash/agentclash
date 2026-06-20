// Pure helpers for constructing and manipulating tool definitions. No React, no
// I/O — fully unit-testable.

import type {
  ComposedDefinition,
  ComposedStep,
  JsonSchemaObject,
  ParamField,
  PrimitiveDefinition,
  ToolDefinition,
  ToolType,
} from "./types";

export const PLACEHOLDER_RE = /\$\{([^}]*)\}/g;

export function emptySchema(): JsonSchemaObject {
  return { type: "object", properties: {}, required: [], additionalProperties: false };
}

export function emptyPrimitiveDefinition(): PrimitiveDefinition {
  return {
    tool_type: "primitive",
    description: "",
    parameters: emptySchema(),
    implementation: { mode: "delegate", primitive: "", args: {} },
  };
}

export function emptyComposedDefinition(): ComposedDefinition {
  return {
    tool_type: "composed",
    description: "",
    parameters: emptySchema(),
    steps: [],
  };
}

export function emptyDefinition(type: ToolType): ToolDefinition {
  return type === "primitive" ? emptyPrimitiveDefinition() : emptyComposedDefinition();
}

/** Convert a JSON Schema object into the friendly param-row representation. */
export function schemaToParams(schema: JsonSchemaObject | undefined): ParamField[] {
  if (!schema || !schema.properties) return [];
  const required = new Set(schema.required ?? []);
  return Object.entries(schema.properties).map(([name, prop]) => ({
    name,
    type: prop.type,
    description: prop.description,
    required: required.has(name),
  }));
}

/** Convert friendly param rows back into a JSON Schema object. */
export function paramsToSchema(params: ParamField[]): JsonSchemaObject {
  const properties: JsonSchemaObject["properties"] = {};
  const required: string[] = [];
  for (const p of params) {
    const name = p.name.trim();
    if (!name) continue;
    properties[name] = { type: p.type };
    if (p.description?.trim()) properties[name].description = p.description.trim();
    if (p.required) required.push(name);
  }
  return { type: "object", properties, required, additionalProperties: false };
}

/** The declared parameter names of a definition. */
export function declaredParamNames(def: ToolDefinition): string[] {
  return Object.keys(def.parameters?.properties ?? {});
}

let stepCounter = 0;
/** Generate a stable-ish step id. Deterministic within a session is enough. */
export function newStepId(existing: ComposedStep[]): string {
  let id: string;
  do {
    stepCounter += 1;
    id = `s${stepCounter}`;
  } while (existing.some((s) => s.id === id));
  return id;
}

/** Move an array item from one index to another (returns a new array). */
export function reorder<T>(items: T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) {
    return items;
  }
  const next = items.slice();
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

/** Extract every ${...} placeholder expression found in a value tree. */
export function placeholdersIn(value: unknown): string[] {
  const out: string[] = [];
  const walk = (node: unknown) => {
    if (typeof node === "string") {
      for (const m of node.matchAll(PLACEHOLDER_RE)) out.push(m[1].trim());
    } else if (Array.isArray(node)) {
      node.forEach(walk);
    } else if (node && typeof node === "object") {
      Object.values(node).forEach(walk);
    }
  };
  walk(value);
  return out;
}

/**
 * Hydrate a possibly-partial stored definition into a complete, editable shape
 * for the given tool type. Tolerates legacy/empty definitions.
 */
export function normalizeDefinition(type: ToolType, raw: unknown): ToolDefinition {
  const base = emptyDefinition(type);
  if (!raw || typeof raw !== "object") return base;
  const r = raw as Record<string, unknown>;

  if (type === "primitive" && base.tool_type === "primitive") {
    const rawImpl = (r.implementation as Record<string, unknown> | undefined) ?? {};
    return {
      tool_type: "primitive",
      description: typeof r.description === "string" ? r.description : base.description,
      parameters: (r.parameters as ToolDefinition["parameters"]) ?? base.parameters,
      implementation: { ...base.implementation, ...rawImpl } as PrimitiveDefinition["implementation"],
    };
  }

  return {
    tool_type: "composed",
    description: typeof r.description === "string" ? r.description : base.description,
    parameters: (r.parameters as ToolDefinition["parameters"]) ?? base.parameters,
    steps: Array.isArray(r.steps) ? (r.steps as ComposedDefinition["steps"]) : [],
  };
}

/** Whether a tool kind can be edited by the visual builder. */
export function isBuilderToolType(kind: string): kind is ToolType {
  return kind === "primitive" || kind === "composed";
}

/** A short human label for a tool type. */
export function toolTypeLabel(type: string): string {
  if (type === "primitive") return "Primitive";
  if (type === "composed") return "Composed";
  return type;
}
