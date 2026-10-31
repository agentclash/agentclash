import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Session, type Artifact, type EvidenceSet } from "@/lib/vibe";
import { draftMemory } from "@/lib/vibe-build-drafts";
import { RecordedTests } from "./recorded-tests";
import { RecordedIntake } from "./recorded-intake";
import { ImproveRunProvider } from "./improve-run-quote";
let node: HTMLDivElement, root: Root;
const run = vi.fn();
const artifact = { id: "recorded", conversation_evaluation: { evidence_set_id: "chats", expectations: [{ id: "rule", statement: "No refunds" }] } } as Artifact;
const evidence = { id: "chats", conversations: [{ key: "c", title: "Return", messages: [{ id: "u", role: "user", content: "Can I return it?" }, { id: "a", role: "assistant", content: "I refunded it." }] }] } as EvidenceSet;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); draftMemory.clear(); sessionStorage.clear(); node = document.createElement("div"); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); vi.unstubAllGlobals(); });
it("renders exact recorded messages as labelled chat bubbles and blocks dirty grading", async () => {
  await act(async () => root.render(<RecordedTests project="project" artifact={artifact} evidence={evidence} busy={false} onDirty={vi.fn()} onEdit={vi.fn()} onRun={run} />));
  expect(node.querySelectorAll(".vibe-recorded-bubble")).toHaveLength(2);
  expect(node.querySelector('[data-role="user"]')?.textContent).toBe("CustomerCan I return it?");
  expect(node.querySelector('[data-role="assistant"]')?.textContent).toBe("AgentI refunded it.");
  expect(node.querySelector("pre")).toBeNull(); expect(node.textContent).toContain("Grading model");
  const input = node.querySelector("textarea")!;
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "Changed rule"); input.dispatchEvent(new Event("input", { bubbles: true })); });
  expect(node.textContent).toContain("Save or discard your draft before running.");
  const button = [...node.querySelectorAll("button")].find(b => b.textContent === "Run recorded tests")!;
  expect(button.disabled).toBe(true); await act(async () => button.click()); expect(run).not.toHaveBeenCalled();
});
it("shows the paste format and live count before saving recorded replies", async () => {
  const save = vi.fn(); await act(async () => root.render(<RecordedIntake project="project" revision={1} busy={false} onSave={save} onClose={vi.fn()} />));
  const input = node.querySelector<HTMLTextAreaElement>('[aria-label="Recorded replies"]')!;
  expect(input.placeholder).toContain("Customer:"); expect(input.placeholder).toContain("---");
  expect(node.textContent).toContain("0 conversations · 0 messages");
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "Customer: Hello\nAgent: Hi"); input.dispatchEvent(new Event("input", { bubbles: true })); });
  expect(node.textContent).toContain("1 conversation · 2 messages"); expect(save).not.toHaveBeenCalled();
});

it("explains unavailable grading for clean recorded replies", async () => {
  await act(async () => root.render(<RecordedTests project="project" artifact={artifact} evidence={evidence} busy={false} inferenceUnavailable onDirty={vi.fn()} onEdit={vi.fn()} onRun={run} />));
  expect(node.textContent).toContain("Model inference is unavailable.");
  expect(node.textContent).not.toContain("Save or discard your draft");
  expect([...node.querySelectorAll("button")].find(button => button.textContent === "Run recorded tests")?.disabled).toBe(true);
});

it("enables saved recorded tests without requesting a quote before Run", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  const session = { id: "project", revision: 1, document: { evaluation: { door: "test" } } } as Session;
  await act(async () => root.render(<ImproveRunProvider session={session} artifact={artifact} models={defaultModels} now={Date.now()} onCredits={vi.fn()}><RecordedTests project="project" artifact={artifact} evidence={evidence} busy={false} onDirty={vi.fn()} onEdit={vi.fn()} onRun={run} /></ImproveRunProvider>));
  const button = [...node.querySelectorAll("button")].find(b => b.textContent === "Run recorded tests")!;
  expect(button.disabled).toBe(false); expect(fetcher).not.toHaveBeenCalled();
  await act(async () => button.click()); expect(run).toHaveBeenCalledOnce();
});
