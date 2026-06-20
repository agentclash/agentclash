// Canonical client-side mirror of backend/internal/toolspec. The builder edits
// these objects directly; they are persisted verbatim as the tool `definition`.

export type ToolType = "primitive" | "composed";

export type JsonSchemaType =
  | "string"
  | "number"
  | "integer"
  | "boolean"
  | "object"
  | "array";

/** A single parameter row, the friendly editor representation of one property. */
export interface ParamField {
  name: string;
  type: JsonSchemaType;
  description?: string;
  required: boolean;
}

/** The JSON Schema object stored in a definition's `parameters`. */
export interface JsonSchemaObject {
  type: "object";
  properties: Record<string, { type: JsonSchemaType; description?: string }>;
  required?: string[];
  additionalProperties?: boolean;
}

export type MockStrategy = "static" | "lookup" | "echo";

export interface MockConfig {
  strategy: MockStrategy;
  response?: unknown;
  lookup_key?: string;
  responses?: Record<string, unknown>;
  template?: unknown;
}

export type PrimitiveMode = "delegate" | "mock";

export interface PrimitiveImplementation {
  mode: PrimitiveMode;
  primitive?: string;
  args?: Record<string, string>;
  mock?: MockConfig;
}

export interface PrimitiveDefinition {
  tool_type: "primitive";
  description?: string;
  parameters: JsonSchemaObject;
  implementation: PrimitiveImplementation;
}

export type StepRefType = "primitive" | "tool";

export interface ComposedStep {
  id: string;
  ref: { type: StepRefType; name: string };
  inputs: Record<string, string>;
}

export interface ComposedDefinition {
  tool_type: "composed";
  description?: string;
  parameters: JsonSchemaObject;
  steps: ComposedStep[];
}

export type ToolDefinition = PrimitiveDefinition | ComposedDefinition;

/** One entry from GET /v1/tool-primitives. */
export interface ToolPrimitive {
  name: string;
  description: string;
  kind: "core" | "file" | "data" | "network" | "build" | "shell";
  parameters: JsonSchemaObject;
  delegatable: boolean;
}

/** A saved workspace tool (GET /v1/workspaces/{id}/tools and /v1/tools/{id}). */
export interface ToolRecord {
  id: string;
  workspace_id?: string;
  name: string;
  slug: string;
  tool_kind: string;
  capability_key: string;
  definition: ToolDefinition | Record<string, unknown>;
  lifecycle_status: string;
  created_at: string;
  updated_at?: string;
}

/** A validation problem surfaced to the user, scoped to a field path. */
export interface ValidationIssue {
  path: string;
  message: string;
}
