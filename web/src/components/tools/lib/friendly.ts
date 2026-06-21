// Plain-language labels and reference helpers. The builder's data model stays in
// engineer terms (primitive / composed / ${placeholders}); everything here is the
// translation layer that lets a non-engineer read and build a tool without ever
// learning that vocabulary or typing template syntax by hand.

import type { JsonSchemaType } from "./types";

/** Friendly name for a JSON Schema type, shown in the inputs editor. */
export function typeLabel(type: JsonSchemaType): string {
  switch (type) {
    case "string":
      return "Text";
    case "number":
      return "Number";
    case "integer":
      return "Whole number";
    case "boolean":
      return "Yes / No";
    case "object":
      return "Group of values";
    case "array":
      return "List";
    default:
      return type;
  }
}

/** The friendly type options offered in the inputs editor, in a sensible order. */
export const TYPE_OPTIONS: { value: JsonSchemaType; label: string }[] = [
  "string",
  "number",
  "integer",
  "boolean",
  "object",
  "array",
].map((t) => ({ value: t as JsonSchemaType, label: typeLabel(t as JsonSchemaType) }));

/** Friendly name for a base operation (primitive). Falls back to the raw name. */
export function operationLabel(name: string): string {
  return OPERATION_LABELS[name] ?? name;
}

// Keys must match the real primitive names from GET /v1/tool-primitives
// (backend/internal/toolspec/catalog.go), or the picker falls back to the raw name.
const OPERATION_LABELS: Record<string, string> = {
  http_request: "Call a web API",
  exec: "Run a command",
  read_file: "Read a file",
  write_file: "Write a file",
  list_files: "List files",
  search_files: "Find files by name",
  search_text: "Search file contents",
  query_sql: "Query a database (SQL)",
  query_json: "Query JSON data",
  run_tests: "Run tests",
  build: "Build the project",
};

/** Friendly heading for a primitive `kind` group. */
export function operationGroupLabel(kind: string): string {
  switch (kind) {
    case "core":
      return "General";
    case "file":
      return "Files";
    case "data":
      return "Data";
    case "network":
      return "Network";
    case "build":
      return "Build";
    case "shell":
      return "Shell";
    default:
      return kind;
  }
}

/** Friendly label + one-liner for each mock strategy. */
export const MOCK_STRATEGY_OPTIONS: {
  value: "static" | "lookup" | "echo";
  label: string;
  hint: string;
}[] = [
  {
    value: "static",
    label: "Always return the same response",
    hint: "Every call gets one fixed response.",
  },
  {
    value: "lookup",
    label: "Return a different response per input",
    hint: "Match one input value to a response, with a fallback.",
  },
  {
    value: "echo",
    label: "Echo the input back",
    hint: "Returns whatever the agent sent, optionally merged with a template.",
  },
];

// --- References: the values a non-engineer can drop into a field --------------

/** One insertable value reference (renders the friendly label, inserts the token). */
export interface ValueReference {
  /** What the user reads, e.g. "order_id". */
  label: string;
  /** A short category for grouping in the menu, e.g. "Agent inputs". */
  group: string;
  /** The token spliced into the field, e.g. "${order_id}". Hidden from the user. */
  token: string;
}

/**
 * References available inside a primitive tool's arguments. Bare `${name}` syntax,
 * plus an "all inputs" object and (for HTTP) secrets.
 */
export function primitiveReferences(paramNames: string[]): ValueReference[] {
  const refs: ValueReference[] = paramNames
    .filter((name) => name.trim())
    .map((name) => ({
      label: name.trim(),
      group: "Agent inputs",
      token: `\${${name.trim()}}`,
    }));
  refs.push({
    label: "All inputs (as one object)",
    group: "Agent inputs",
    token: "${parameters}",
  });
  return refs;
}

/**
 * References available inside a composed step's inputs: `${params.x}` for the
 * tool's own inputs and `${steps.ID.output}` for the result of an earlier step.
 */
export function stepReferences(
  paramNames: string[],
  earlierStepIds: string[],
  stepNumberById: Map<string, number>,
): ValueReference[] {
  const refs: ValueReference[] = paramNames.map((name) => ({
    label: name,
    group: "Agent inputs",
    token: `\${params.${name}}`,
  }));
  for (const id of earlierStepIds) {
    const n = stepNumberById.get(id);
    refs.push({
      label: n ? `Result of step ${n}` : `Result of ${id}`,
      group: "Earlier steps",
      token: `\${steps.${id}.output}`,
    });
  }
  return refs;
}
