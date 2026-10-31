import { draftMemory } from "@/lib/vibe-build-drafts";
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
  draftMemory.clear(); sessionStorage.clear();
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

it("offers running and model context before a long list of tests", async () => {
  artifact.blueprint = { cases: Array.from({ length: 50 }, (_, index) => ({ key: `case-${index}`, payload: { question: `Question ${index}?` } })) };
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    modelLabel="Returns model" onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  const firstCase = node.querySelector(".vibe-result-row summary")!;
  const runButton = button("Run 50 tests");
  const scope = [...node.querySelectorAll("p")].find(item => item.textContent?.includes("Returns model"))!;
  expect(runButton.disabled).toBe(false);
  expect(runButton.compareDocumentPosition(firstCase) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(scope.compareDocumentPosition(firstCase) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(scope.textContent).toContain("Your live app is not called.");
  expect(run).not.toHaveBeenCalled();
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
  expect(node.querySelector("h2")?.textContent).toBe("5 tests need review");
  expect(button("Run 5 tests").disabled).toBe(true);
  expect(button("Keep these tests").disabled).toBe(true);
  await act(async () => button("Run 5 tests").click());
  expect(run).not.toHaveBeenCalled();
});

it.each([false, true])("keeps rules available and expands them only when policy needs attention (%s)", async pendingPolicy => {
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    pendingPolicy={pendingPolicy}
    rules={[{ id: "window", statement: "Only unopened purchases within 30 days qualify." }]}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  const summary = [...node.querySelectorAll("summary")].find(item => item.textContent === "Rules being tested")!;
  expect(summary).toBeDefined();
  const disclosure = summary.closest("details")!;
  expect(disclosure.open).toBe(pendingPolicy);
  await act(async () => summary.click());
  expect(disclosure.open).toBe(!pendingPolicy);
  expect(disclosure.textContent).toContain("Only unopened purchases within 30 days qualify.");
  expect(run).not.toHaveBeenCalled();
});

it("lets someone without an agent keep prepared tests without offering a fabricated run", async () => {
  artifact.agent_prompt = "";
  const save = vi.fn();
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={save} onSettings={vi.fn()} />));
  await act(async () => button("Keep for later").click());
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  expect(node.querySelector(".vibe-button-primary")?.textContent).toContain("Keep these tests");
  expect(node.querySelector('[aria-label="Agent instructions"]')).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(run).not.toHaveBeenCalled();
  await act(async () => button("Keep these tests").click());
  expect(save).toHaveBeenCalledOnce();
});

it("edits one expectation through its stable case key, preserving other inputs and custom grading", async () => {
  artifact.blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [
    { key: "first", payload: { question: "First input" }, expectations: [{ key: "expected_behavior", kind: "text", value: "First expectation" }] },
    { key: "custom", payload: { question: "Keep custom grading" }, expectations: [{ key: "custom_rule", kind: "regex", value: "^safe$" }] },
  ] };
  const before = structuredClone(artifact); const edit = vi.fn().mockResolvedValue(true); const select = vi.fn();
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    onSelectCase={select}
    onRun={run} onEdit={edit} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  const firstCase = node.querySelector<HTMLDetailsElement>(".vibe-result-row")!;
  expect(firstCase.open).toBe(false);
  await act(async () => firstCase.querySelector("summary")!.click());
  expect(firstCase.open).toBe(true);
  expect(node.querySelector("textarea")).toBeNull();
  expect(select).not.toHaveBeenCalled(); expect(edit).not.toHaveBeenCalled(); expect(run).not.toHaveBeenCalled();
  await act(async () => button("Edit this expectation").click());
  expect(select).toHaveBeenCalledWith("first");
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

it("restores unsaved expectations after unmount and requires review only when their actual source changes", async () => {
  artifact.blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [{ key: "boundary", payload: { question: "Opened item?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Decline." }] }] };
  await render(); await act(async () => button("Edit tests").click());
  const expected = node.querySelectorAll("textarea")[1];
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(expected, "Explain the policy.");
    expected.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => root.unmount()); root = createRoot(node); await render();
  expect(node.querySelectorAll("textarea")[1].value).toBe("Explain the policy.");
  artifact = { ...artifact, validation: { status: "supported", blueprint_hash: "validation-only", policy_hash: "policy", validator_version: "v2" } };
  await render(); expect(node.querySelector('[role="alert"]')).toBeNull();
  const oldSource = artifact.blueprint as { judges: unknown; cases: unknown };
  artifact = { ...artifact, blueprint: { cases: oldSource.cases, judges: oldSource.judges } };
  await render(); expect(node.querySelector('[role="alert"]')).toBeNull();
  artifact = { ...artifact, blueprint: { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [{ key: "boundary", payload: { question: "Opened item?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Ask purchase age." }] }] } };
  await render();
  expect(node.querySelector('[role="alert"]')?.textContent).toContain("saved source changed");
  expect(button("Save test changes").disabled).toBe(true);
  await act(async () => button("Keep my draft for this source").click());
  expect(button("Save test changes").disabled).toBe(false);
  expect(node.querySelectorAll("textarea")[1].value).toBe("Explain the policy.");
  await act(async () => button("Cancel").click());
  expect(node.querySelector("textarea")).toBeNull();
});


it("reconciles same-artifact case history with the visible expectation while preserving drafts", async () => {
  artifact.blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [
    { key: "a", payload: { question: "A input" }, expectations: [{ key: "expected_behavior", kind: "text", value: "A expectation" }] },
    { key: "b", payload: { question: "B input" }, expectations: [{ key: "expected_behavior", kind: "text", value: "B expectation" }] },
  ] };
  const select = async (selectedCase?: string) => act(async () => root.render(<TestSuitePanel projectID="history-project" selectedCase={selectedCase} onSelectCase={vi.fn()} artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  await select("a");
  const area = node.querySelector("textarea")!;
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(area, "A unsent change"); area.dispatchEvent(new Event("input", { bubbles: true })); });
  await select(undefined);
  expect(node.querySelector("textarea")).toBeNull();
  await select("b"); expect(node.querySelector("textarea")?.value).toBe("B expectation");
  await select("a"); expect(node.querySelector("textarea")?.value).toBe("A unsent change");
  await select("b"); expect(node.querySelector("textarea")?.value).toBe("B expectation");
  expect(run).not.toHaveBeenCalled();
});

it("collects imported agent instructions inline with one primary and a quiet Keep for later", async () => {
  artifact.agent_prompt = ""; const edit = vi.fn().mockResolvedValue(true); const save = vi.fn(); const agent = vi.fn();
  await act(async () => root.render(<TestSuitePanel mode="tests" onAgent={agent} artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={edit} onDirty={vi.fn()} onSave={save} onSettings={vi.fn()} />));
  const input = node.querySelector<HTMLTextAreaElement>('[aria-label="Agent instructions"]');
  expect(input).not.toBeNull();
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  expect(button("Add your agent's instructions").disabled).toBe(true);
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "Our actual policy"); input!.dispatchEvent(new Event("input", { bubbles: true })); });
  await act(async () => node.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(edit).toHaveBeenCalledWith({ artifact_id: artifact.id, agent_prompt: "Our actual policy" });
  expect(agent).not.toHaveBeenCalled(); expect(run).not.toHaveBeenCalled();
  expect(save).not.toHaveBeenCalled();
  expect(node.querySelector('[aria-label="Agent instructions"]')).toBeNull();
  await act(async () => button("Keep these tests").click()); expect(save).toHaveBeenCalledOnce();
});

it("explains unsaved and unavailable runs next to the action", async () => {
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked comparison={false}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  expect(node.textContent).toContain("Save or discard your draft before running.");
  expect(button("Run 5 tests").disabled).toBe(true);
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} inferenceUnavailable comparison={false}
    onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  expect(node.textContent).toContain("Model inference is unavailable.");
});

it("keeps imported tests visible and preserves the instruction draft while editing one expectation", async () => {
  artifact.agent_prompt = "";
  artifact.blueprint = { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [{ key: "one", payload: { question: "Opened item?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Decline" }] }] };
  await render();
  const input = node.querySelector<HTMLTextAreaElement>('[aria-label="Agent instructions"]')!;
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "Retain my policy"); input.dispatchEvent(new Event("input", { bubbles: true })); });
  const row = node.querySelector<HTMLDetailsElement>(".vibe-result-row"); expect(row).not.toBeNull();
  await act(async () => row!.querySelector<HTMLElement>("summary")!.click());
  await act(async () => button("Edit this expectation").click());
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  expect(node.querySelector('[aria-label="Agent instructions"]')).toBeNull();
  await act(async () => button("Cancel").click());
  expect(node.querySelector<HTMLTextAreaElement>('[aria-label="Agent instructions"]')?.value).toBe("Retain my policy");
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
});


it("dismisses imported instructions without saving or discarding the instruction draft", async () => {
  artifact.agent_prompt = ""; const save = vi.fn(); const edit = vi.fn();
  await act(async () => root.render(<TestSuitePanel artifact={artifact} busy={false} blocked={false} comparison={false}
    onRun={run} onEdit={edit} onDirty={vi.fn()} onSave={save} onSettings={vi.fn()} />));
  const input = node.querySelector<HTMLTextAreaElement>('[aria-label="Agent instructions"]')!;
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "Keep my unsaved policy"); input.dispatchEvent(new Event("input", { bubbles: true })); });
  await act(async () => button("Keep for later").click());
  expect(node.querySelector('[aria-label="Agent instructions"]')).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(edit).not.toHaveBeenCalled(); expect(run).not.toHaveBeenCalled();
  expect(button("Keep for later")).toBeUndefined();
  expect(button("Keep these tests").disabled).toBe(false);
  await act(async () => button("Add your agent").click());
  expect(node.querySelector<HTMLTextAreaElement>('[aria-label="Agent instructions"]')?.value).toBe("Keep my unsaved policy");
});

it("U8 explains the friendly model used for the text run", async () => {
  await act(async () => root.render(<TestSuitePanel artifact={artifact} mode="tests" busy={false} blocked={false} comparison={false} modelLabel="openai/gpt-4.1-mini" onRun={run} onEdit={vi.fn()} onDirty={vi.fn()} onSave={vi.fn()} onSettings={vi.fn()} />));
  expect(node.textContent).toContain("Runs your instructions on GPT-4.1 mini. Your live app is not called.");
  expect(node.textContent).not.toContain("openai/gpt-4.1-mini");
});
