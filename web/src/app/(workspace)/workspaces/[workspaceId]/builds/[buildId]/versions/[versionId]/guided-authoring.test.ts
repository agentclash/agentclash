import { describe, expect, it } from "vitest";

import {
  guidedStateFromVersion,
  specsFromGuidedState,
  versionPayloadFromTemplate,
} from "./guided-authoring";

describe("versionPayloadFromTemplate", () => {
  it("creates a starter payload for research analyst", () => {
    const payload = versionPayloadFromTemplate("research_analyst");

    expect(payload.agent_kind).toBe("llm_agent");
    expect(payload.policy_spec).toMatchObject({
      role: "You are a meticulous research analyst.",
    });
    expect(payload.workflow_spec).toMatchObject({
      tool_strategy: "prefer_tools_first",
    });
    expect(payload.output_schema).toMatchObject({
      type: "object",
      required: ["answer", "summary"],
    });
  });
});

describe("guidedStateFromVersion", () => {
  it("hydrates guided fields from an existing version", () => {
    const guided = guidedStateFromVersion({
      agent_kind: "workflow_agent",
      interface_spec: { primary_input: "support_ticket" },
      policy_spec: {
        role: "You are a triage agent.",
        system_prompt: "Stay calm.",
        instructions: "Classify the issue.",
        success_conditions: "Return urgency and next step.",
      },
      reasoning_spec: {},
      memory_spec: { strategy: "session" },
      workflow_spec: { tool_strategy: "prefer_tools_first" },
      guardrail_spec: {},
      model_spec: {},
      output_schema: {
        type: "object",
        properties: {
          answer: { type: "string" },
          summary: { type: "string" },
          citations: { type: "array", items: { type: "string" } },
        },
      },
      trace_contract: {},
      publication_spec: {},
    });

    expect(guided).toMatchObject({
      agentKind: "workflow_agent",
      primaryInput: "support_ticket",
      toolStrategy: "prefer_tools_first",
      memoryMode: "session",
      outputMode: "answer_summary_citations",
    });
  });

  it("treats unknown output schema as custom", () => {
    const guided = guidedStateFromVersion({
      agent_kind: "llm_agent",
      interface_spec: {},
      policy_spec: { instructions: "Do the thing." },
      reasoning_spec: {},
      memory_spec: {},
      workflow_spec: {},
      guardrail_spec: {},
      model_spec: {},
      output_schema: { type: "object", properties: { verdict: { type: "boolean" } } },
      trace_contract: {},
      publication_spec: {},
    });

    expect(guided.outputMode).toBe("custom");
  });
});

describe("specsFromGuidedState", () => {
  it("preserves unknown keys while writing guided fields", () => {
    const updated = specsFromGuidedState(
      {
        agentKind: "llm_agent",
        role: "You are a reviewer.",
        systemPrompt: "Be brief.",
        instructions: "Inspect the diff.",
        successConditions: "Return the highest priority issue first.",
        primaryInput: "pull_request_diff",
        toolStrategy: "use_when_helpful",
        memoryMode: "extended",
        outputMode: "answer_object",
      },
      {
        agent_kind: "workflow_agent",
        interface_spec: { primary_input: "old_value", untouched: true },
        policy_spec: { legacy: "keep-me" },
        reasoning_spec: {},
        memory_spec: { another_key: "keep-me" },
        workflow_spec: { untouched: true },
        guardrail_spec: {},
        model_spec: {},
        output_schema: { existing: "keep-me" },
        trace_contract: {},
        publication_spec: {},
      },
    );

    expect(updated.agent_kind).toBe("llm_agent");
    expect(updated.interface_spec).toMatchObject({
      primary_input: "pull_request_diff",
      untouched: true,
    });
    expect(updated.policy_spec).toMatchObject({
      legacy: "keep-me",
      role: "You are a reviewer.",
      instructions: "Inspect the diff.",
    });
    expect(updated.memory_spec).toMatchObject({
      another_key: "keep-me",
      strategy: "extended",
    });
    expect(updated.output_schema).toMatchObject({
      existing: "keep-me",
      type: "object",
      properties: {
        answer: { type: "string" },
      },
    });
  });
});
