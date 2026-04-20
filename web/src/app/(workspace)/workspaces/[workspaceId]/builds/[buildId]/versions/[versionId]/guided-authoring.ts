import type { AgentBuildVersion, AgentKind } from "@/lib/api/types";

export type OutputMode =
  | "freeform_text"
  | "answer_object"
  | "answer_summary_citations"
  | "custom";

export type ToolStrategy =
  | "manual_only"
  | "use_when_helpful"
  | "prefer_tools_first";

export type MemoryMode = "none" | "session" | "extended";

export interface GuidedTemplate {
  id: string;
  name: string;
  summary: string;
  guided: GuidedAuthoringState;
}

export interface GuidedAuthoringState {
  agentKind: AgentKind;
  role: string;
  systemPrompt: string;
  instructions: string;
  successConditions: string;
  primaryInput: string;
  toolStrategy: ToolStrategy;
  memoryMode: MemoryMode;
  outputMode: OutputMode;
}

export type EditableSpecs = Pick<
  AgentBuildVersion,
  | "agent_kind"
  | "interface_spec"
  | "policy_spec"
  | "reasoning_spec"
  | "memory_spec"
  | "workflow_spec"
  | "guardrail_spec"
  | "model_spec"
  | "output_schema"
  | "trace_contract"
  | "publication_spec"
>;

const blankGuidedState: GuidedAuthoringState = {
  agentKind: "llm_agent",
  role: "",
  systemPrompt: "",
  instructions: "",
  successConditions: "",
  primaryInput: "user_request",
  toolStrategy: "use_when_helpful",
  memoryMode: "none",
  outputMode: "freeform_text",
};

export const guidedTemplates: GuidedTemplate[] = [
  {
    id: "blank",
    name: "Blank Starter",
    summary: "Start clean, but with guided fields instead of a raw JSON wall.",
    guided: blankGuidedState,
  },
  {
    id: "general_assistant",
    name: "General Assistant",
    summary: "A safe default for broad task execution with tools when they help.",
    guided: {
      agentKind: "llm_agent",
      role: "You are a practical AI teammate for a product team.",
      systemPrompt:
        "Favor clear, direct answers. Use tools when they materially improve accuracy.",
      instructions:
        "Understand the user's goal, gather what you need, and complete the task with concise explanations and concrete next steps.",
      successConditions:
        "The final answer is correct, actionable, and easy for a non-expert to use immediately.",
      primaryInput: "user_request",
      toolStrategy: "use_when_helpful",
      memoryMode: "session",
      outputMode: "answer_object",
    },
  },
  {
    id: "research_analyst",
    name: "Research Analyst",
    summary: "Good for synthesis, comparison, and evidence-backed recommendations.",
    guided: {
      agentKind: "llm_agent",
      role: "You are a meticulous research analyst.",
      systemPrompt:
        "Prioritize evidence, call out uncertainty, and avoid overstating conclusions.",
      instructions:
        "Investigate the request, compare competing options, and produce a recommendation backed by traceable evidence.",
      successConditions:
        "The final answer includes the recommendation, key supporting evidence, and the main uncertainty or risk to watch.",
      primaryInput: "research_question",
      toolStrategy: "prefer_tools_first",
      memoryMode: "session",
      outputMode: "answer_summary_citations",
    },
  },
  {
    id: "support_triage",
    name: "Support Triage",
    summary: "Classify urgency, explain the issue, and suggest a next action.",
    guided: {
      agentKind: "workflow_agent",
      role: "You are a support triage agent for incoming customer issues.",
      systemPrompt:
        "Stay calm, categorize the issue, and surface the next best action quickly.",
      instructions:
        "Read the incoming report, determine urgency, summarize the core problem, and recommend the next operational step.",
      successConditions:
        "The result clearly labels urgency, explains why, and suggests an actionable next step.",
      primaryInput: "support_ticket",
      toolStrategy: "use_when_helpful",
      memoryMode: "none",
      outputMode: "answer_object",
    },
  },
  {
    id: "structured_extractor",
    name: "Structured Extractor",
    summary: "Turn messy text into a dependable structured response payload.",
    guided: {
      agentKind: "programmatic_agent",
      role: "You are a structured extraction agent.",
      systemPrompt:
        "Return only what the schema calls for and avoid speculative fields.",
      instructions:
        "Read the input carefully and extract the required fields into a structured response.",
      successConditions:
        "The response is structured, faithful to the source, and contains no invented values.",
      primaryInput: "source_document",
      toolStrategy: "manual_only",
      memoryMode: "none",
      outputMode: "answer_object",
    },
  },
];

export function getGuidedTemplate(templateID: string): GuidedTemplate {
  return (
    guidedTemplates.find((template) => template.id === templateID) ??
    guidedTemplates[0]
  );
}

export function guidedStateFromVersion(
  version: EditableSpecs,
): GuidedAuthoringState {
  const policySpec = asObject(version.policy_spec);
  const interfaceSpec = asObject(version.interface_spec);
  const workflowSpec = asObject(version.workflow_spec);
  const memorySpec = asObject(version.memory_spec);

  return {
    agentKind: isAgentKind(version.agent_kind)
      ? version.agent_kind
      : blankGuidedState.agentKind,
    role: readString(policySpec.role),
    systemPrompt: readString(policySpec.system_prompt),
    instructions: readString(policySpec.instructions),
    successConditions: readString(policySpec.success_conditions),
    primaryInput:
      readString(interfaceSpec.primary_input) || blankGuidedState.primaryInput,
    toolStrategy: parseToolStrategy(workflowSpec.tool_strategy),
    memoryMode: parseMemoryMode(memorySpec.strategy),
    outputMode: inferOutputMode(version.output_schema),
  };
}

export function specsFromGuidedState(
  guided: GuidedAuthoringState,
  existing: EditableSpecs,
): EditableSpecs {
  const policySpec = writeStringKeys(asObject(existing.policy_spec), {
    role: guided.role,
    system_prompt: guided.systemPrompt,
    instructions: guided.instructions,
    success_conditions: guided.successConditions,
  });

  const interfaceSpec = writeStringKeys(asObject(existing.interface_spec), {
    primary_input: guided.primaryInput,
  });

  const workflowSpec = writeStringKeys(asObject(existing.workflow_spec), {
    tool_strategy: guided.toolStrategy,
  });

  const memorySpec = writeStringKeys(asObject(existing.memory_spec), {
    strategy: guided.memoryMode,
  });

  return {
    ...existing,
    agent_kind: guided.agentKind,
    policy_spec: policySpec,
    interface_spec: interfaceSpec,
    workflow_spec: workflowSpec,
    memory_spec: memorySpec,
    output_schema: outputSchemaForMode(
      guided.outputMode,
      asObject(existing.output_schema),
    ),
  };
}

export function versionPayloadFromTemplate(templateID: string): EditableSpecs {
  const template = getGuidedTemplate(templateID);
  return specsFromGuidedState(template.guided, {
    agent_kind: template.guided.agentKind,
    interface_spec: {},
    policy_spec: {},
    reasoning_spec: {},
    memory_spec: {},
    workflow_spec: {},
    guardrail_spec: {},
    model_spec: {},
    output_schema: {},
    trace_contract: {},
    publication_spec: {},
  });
}

function outputSchemaForMode(
  mode: OutputMode,
  existing: Record<string, unknown>,
): Record<string, unknown> {
  switch (mode) {
    case "freeform_text":
      return {};
    case "answer_object":
      return {
        ...existing,
        type: "object",
        properties: {
          answer: { type: "string" },
        },
        required: ["answer"],
      };
    case "answer_summary_citations":
      return {
        ...existing,
        type: "object",
        properties: {
          answer: { type: "string" },
          summary: { type: "string" },
          citations: {
            type: "array",
            items: { type: "string" },
          },
        },
        required: ["answer", "summary"],
      };
    case "custom":
      return existing;
    default:
      return existing;
  }
}

function inferOutputMode(outputSchema: unknown): OutputMode {
  const schema = asObject(outputSchema);
  if (Object.keys(schema).length === 0) {
    return "freeform_text";
  }

  const properties = asObject(schema.properties);
  if (
    schema.type === "object" &&
    isStringSchema(properties.answer) &&
    Object.keys(properties).length === 1
  ) {
    return "answer_object";
  }

  if (
    schema.type === "object" &&
    isStringSchema(properties.answer) &&
    isStringSchema(properties.summary) &&
    isStringArraySchema(properties.citations)
  ) {
    return "answer_summary_citations";
  }

  return "custom";
}

function isStringSchema(value: unknown): boolean {
  const schema = asObject(value);
  return schema.type === "string";
}

function isStringArraySchema(value: unknown): boolean {
  const schema = asObject(value);
  const items = asObject(schema.items);
  return schema.type === "array" && items.type === "string";
}

function asObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  return { ...(value as Record<string, unknown>) };
}

function writeStringKeys(
  base: Record<string, unknown>,
  values: Record<string, string>,
): Record<string, unknown> {
  const next = { ...base };
  for (const [key, value] of Object.entries(values)) {
    const trimmed = value.trim();
    if (trimmed) {
      next[key] = trimmed;
    } else {
      delete next[key];
    }
  }
  return next;
}

function readString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function parseToolStrategy(value: unknown): ToolStrategy {
  if (
    value === "manual_only" ||
    value === "use_when_helpful" ||
    value === "prefer_tools_first"
  ) {
    return value;
  }
  return blankGuidedState.toolStrategy;
}

function parseMemoryMode(value: unknown): MemoryMode {
  if (value === "none" || value === "session" || value === "extended") {
    return value;
  }
  return blankGuidedState.memoryMode;
}

function isAgentKind(value: string): value is AgentKind {
  return (
    value === "llm_agent" ||
    value === "workflow_agent" ||
    value === "programmatic_agent" ||
    value === "multi_agent_system" ||
    value === "hosted_external"
  );
}
