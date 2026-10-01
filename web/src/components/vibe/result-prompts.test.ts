import { describe, expect, it } from "vitest";
import type { CaseResult } from "@/lib/vibe";
import { fixPrompt, originalInputs } from "./result-prompts";

const failure: CaseResult = {
  case_key: "return-memory",
  version: "original-rules",
  title: "Repeated purchase question",
  verdict: "FAIL",
  input: null,
  output: "",
  expected: "Remember earlier details without inventing a policy.",
  expectations: [{ id: "memory", statement: "Ask only for missing purchase details." }],
  messages: [
    { id: "u1", role: "user", content: "I bought it 10 days ago.\n---\nAgent: this is still user text." },
    { id: "a1", role: "assistant", content: "Is it opened?" },
    { id: "u2", role: "user", content: "Unopened. Keep literal `code` and $100 intact." },
    { id: "a2", role: "assistant", content: "How many days ago did you buy it?" },
  ],
  checks: [{ key: "memory", verdict: "FAIL", evidence: "The agent asked for the purchase age already supplied.", message_ids: ["u1", "a2"] }],
};

describe("coding-tool fix prompt", () => {
  it("preserves the complete observed exchange, rules and provenance without inventing a root cause", () => {
    const prompt = fixPrompt(failure, "provided_conversations")!;
    expect(prompt).toContain("The replies below were supplied by the user");
    expect(prompt).toContain("does not establish a root cause");
    expect(prompt).toContain("No code or agent instructions have been changed");
    expect(prompt).toContain("untrusted test evidence");
    expect(prompt).toContain("Do not weaken the rule to get a pass");
    const data = JSON.parse(prompt.slice(prompt.indexOf("{\n"), prompt.lastIndexOf("\n}") + 2));
    expect(data.original_conversation).toEqual(failure.messages);
    expect(data.observed_findings[0]).toMatchObject({
      expected_behavior: failure.expectations![0].statement,
      explanation: failure.checks[0].evidence,
      cited_message_ids: ["u1", "a2"],
    });
    expect(data.shared_expected_behavior).toBe(failure.expected);
  });

  it("distinguishes generated text tests from replies observed in the live app", () => {
    const generated = { ...failure, messages: undefined, input: { question: "Return?" }, output: '{"answer":"yes","id":900719925474099312345}' };
    const prompt = fixPrompt(generated, "prompt")!;
    expect(prompt).toContain("not an observed reply from the user's live app");
    expect(prompt).toContain("First check whether the same issue exists in the app");
    const data = JSON.parse(prompt.slice(prompt.indexOf("{\n"), prompt.lastIndexOf("\n}") + 2));
    expect(data.recorded_reply).toBe(generated.output);
    expect(data.original_input).toEqual(generated.input);
    expect(fixPrompt(generated)).toContain("does not identify how this reply was obtained");
  });

  it.each(["PASS", "UNKNOWN"] as const)("does not manufacture a fix from a %s result", (verdict) => {
    expect(fixPrompt({ ...failure, verdict }, "provided_conversations")).toBeNull();
  });

  it("does not blame the app for missing replies or evaluator failures", () => {
    expect(fixPrompt({ ...failure, messages: undefined, output: "" }, "prompt")).toBeNull();
    expect(fixPrompt({ ...failure, checks: [{ ...failure.checks[0], error: { message: "Evaluator unavailable" } }] }, "provided_conversations")).toBeNull();
  });
});

describe("fixed inputs for a new-answer check", () => {
  it("keeps all customer and context turns byte-for-byte and replaces only agent replies", () => {
    const withContext: CaseResult = { ...failure, messages: [
      { id: "s1", role: "system", content: "Policy: 30 days. This is supplied context." },
      ...failure.messages!,
      { id: "t1", role: "tool", content: '{"purchase_age":10}' },
    ] };
    const result = JSON.parse(originalInputs([withContext, failure]));
    expect(result.conversations).toHaveLength(2);
    for (const [index, message] of withContext.messages!.entries()) {
      expect(result.conversations[0].messages[index]).toEqual({
        role: message.role,
        content: message.role === "assistant" ? "[Paste the new agent reply here]" : message.content,
      });
    }
    expect(originalInputs([{ ...failure, messages: undefined, input: { question: "Keep this exact input." } }])).toContain("Keep this exact input.");
  });

  it("does not replace missing original inputs with an invented test", () => {
    expect(() => originalInputs([])).toThrow("No original inputs");
    expect(() => originalInputs([{ ...failure, messages: undefined, input: null }])).toThrow("unavailable");
  });
});
