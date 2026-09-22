import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { instructionDiff } from "./instruction-diff";
import { changedCases, suiteCases } from "./test-suite-cases";
import { PromptChange } from "./prompt-change";
import { conversationActivity } from "./activity-status";
import { defaultModels, type Operation } from "@/lib/vibe";

it.each([
  ["Allow opened returns within 30 days.", "Allow unopened returns within 30 days."],
  ["Ask for age.", "Ask for age?"],
  ["Ask for age and condition.", "Ask for age."],
  ["Ask for age. Ask for age.", "Ask for age. Ask for condition."],
  ["Rule one.\nRule two.\n", "Rule one.\nNew rule two!\n"],
  ["Exact same policy", "Exact same policy"],
])("retains exact instructions in the word diff: %s", (before, after) => {
  const parts = instructionDiff(before, after);
  expect(parts.filter(p => p.kind !== "added").map(p => p.text).join("")).toBe(before);
  expect(parts.filter(p => p.kind !== "removed").map(p => p.text).join("")).toBe(after);
  if (before.includes("opened returns")) expect(parts.filter(p => p.kind === "removed")).toEqual([{ kind: "removed", text: "opened" }]);
});

it("preserves five imported case identities and emits only changed fields", () => {
  const blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: Array.from({ length: 5 }, (_, i) => ({ key: `custom-${i}`, payload: { question: `Input ${i}`, extra: "preserved" }, expectations: [{ key: "expected_behavior", kind: "text", value: `Expected ${i}` }] })) };
  const rows = suiteCases(blueprint);
  expect(rows).toHaveLength(5);
  expect(changedCases(rows, rows.map((r, i) => i === 3 ? { ...r, expected: "Corrected expectation" } : r))).toEqual([{ action: "update", case_key: "custom-3", expected: "Corrected expectation" }]);
  expect(changedCases(rows, rows)).toEqual([]);
  expect(suiteCases({ cases: [{ key: "structured", payload: { text: "custom" } }] })).toEqual([{ key: "structured", input: '{\n  "text": "custom"\n}', expected: "Uses the grading rules in your imported pack.", editable: false }]);
});

it("copies the complete updated policy and resets feedback on a new version", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const copy = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: copy } });
  const node = document.createElement("div"), root = createRoot(node);
  const before = "Allow opened returns within 30 days. Never process refunds.";
  const after = before.replace("opened", "unopened");
  try {
    await act(async () => root.render(<PromptChange key="one" before={before} after={after} />));
    await act(async () => node.querySelector("button")!.click());
    expect(copy).toHaveBeenCalledWith(after);
    expect(node.textContent).toContain("Copied instructions");
    await act(async () => root.render(<PromptChange key="two" before={after} after={after.replace("30", "45")} />));
    expect(node.textContent).not.toContain("Copied instructions");
  } finally { await act(async () => root.unmount()); vi.unstubAllGlobals(); }
});

it("keeps conversation, queue, stopping and counted test status truthful", () => {
  const operation: Operation = { id: "op", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] };
  expect(conversationActivity(operation, true)).toBe("Thinking…");
  expect(conversationActivity({ ...operation, conversation_decision: { intent: "chat", source_message_id: "message" } }, true)).toBe("Thinking…");
  expect(conversationActivity({ ...operation, state: "QUEUED" }, true)).toBe("Waiting to start…");
  expect(conversationActivity({ ...operation, state: "CANCELLING" }, true)).toBe("Stopping…");
  expect(conversationActivity({ ...operation, conversation_decision: { intent: "edit_tests", source_message_id: "message" } }, true)).toBe("Preparing your tests…");
});
