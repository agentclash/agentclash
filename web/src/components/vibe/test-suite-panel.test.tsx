import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { Artifact } from "@/lib/vibe";
import { TestSuitePanel } from "./test-suite-panel";

let node: HTMLDivElement;
let root: Root;
let artifact: Artifact;
const run = vi.fn();

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  node = document.createElement("div");
  root = createRoot(node);
  artifact = {
    id: "tests", kind: "test_suite", title: "Returns", agent_prompt: "Follow the return policy.",
    accepted: false, source_message_id: "request", summary: "Return eligibility.",
    blueprint: { cases: Array.from({ length: 5 }, (_, index) => ({ key: `case-${index}`, payload: { question: `Question ${index}?` } })) },
  };
  run.mockClear();
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});
async function render(pendingPolicy = false) {
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    pendingPolicy={pendingPolicy} onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
}
function button(label: string) {
  return [...node.querySelectorAll<HTMLButtonElement>("button")].find(item => item.textContent?.trim() === label)!;
}

it("counts the retained cases and leaves legacy imported tests available without a validation claim", async () => {
  await render();
  expect(node.querySelector("h2")?.textContent).toBe("5 tests are ready");
  expect(button("Run 5 tests").disabled).toBe(false);
  await act(async () => button("Run 5 tests").click());
  expect(run).toHaveBeenCalledOnce();
});

it("labels a failed policy update as unapplied and makes running the previous tests explicit", async () => {
  await render(true);
  expect(node.querySelector("h2")?.textContent).toBe("5 tests from your previous policy");
  expect(node.textContent).toContain("Your previous tests are unchanged. This update hasn’t been applied.");
  expect(button("Run previous tests").disabled).toBe(false);
  expect(button("Run 5 tests")).toBeUndefined();
});

it.each(["contradicted", "unclear", "unavailable"] as const)("does not offer an unsupported %s candidate as ready", async status => {
  artifact.validation = { status, blueprint_hash: "suite", policy_hash: "policy", validator_version: "v1" };
  await render();
  expect(node.querySelector("h2")?.textContent).toBe("5 tests need checking");
  expect(button("Run 5 tests").disabled).toBe(true);
  expect(button("Keep these tests").disabled).toBe(true);
  await act(async () => button("Run 5 tests").click());
  expect(run).not.toHaveBeenCalled();
});

it("keeps the tested rules in a collapsed, optional disclosure", async () => {
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    rules={[{ id: "window", statement: "Only unopened purchases within 30 days qualify." }]}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  const summary = [...node.querySelectorAll("summary")].find(item => item.textContent === "Rules being tested")!;
  expect(summary).toBeDefined();
  const disclosure = summary.closest("details")!;
  expect(disclosure.open).toBe(false);
  await act(async () => summary.click());
  expect(disclosure.open).toBe(true);
  expect(disclosure.textContent).toContain("Only unopened purchases within 30 days qualify.");
});

it("lets someone without an agent keep prepared tests without offering a fabricated run", async () => {
  artifact.agent_prompt = "";
  const save = vi.fn();
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={save} onSettings={vi.fn()} />));
  await act(async () => button("Keep for later").click());
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  expect(node.querySelector(".vibe-button-primary")?.textContent).toContain("Keep these tests");
  expect(run).not.toHaveBeenCalled(); expect(save).not.toHaveBeenCalled();
  await act(async () => button("Keep these tests").click());
  expect(save).toHaveBeenCalledOnce(); expect(run).not.toHaveBeenCalled();
});

it("edits one expectation through its stable case key, preserving other inputs and custom grading", async () => {
  artifact.blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [
    { key: "first", payload: { question: "First input" }, expectations: [{ key: "expected_behavior", kind: "text", value: "First expectation" }] },
    { key: "custom", payload: { question: "Keep custom grading" }, expectations: [{ key: "custom_rule", kind: "regex", value: "^safe$" }] },
  ] };
  const before = structuredClone(artifact); const edit = vi.fn().mockResolvedValue(true);
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={edit} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  await act(async () => button("Edit this expectation").click());
  expect(node.querySelectorAll("textarea")).toHaveLength(1);
  const area = node.querySelector("textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(area, "Updated expectation");
    area.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => button("Save test changes").click());
  expect(edit).toHaveBeenCalledWith({ artifact_id: "tests", case_changes: [{ action: "update", case_key: "first", expected: "Updated expectation" }] });
  expect(artifact).toEqual(before); expect(run).not.toHaveBeenCalled();
});
