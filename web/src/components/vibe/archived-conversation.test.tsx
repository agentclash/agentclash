import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ArchivedConversation } from "./archived-conversation";
import { defaultModels, type Session } from "@/lib/vibe";

let node: HTMLDivElement;
let root: Root;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  node = document.createElement("div");
  document.body.appendChild(node);
  root = createRoot(node);
});
afterEach(async () => {
  await act(async () => root.unmount());
  node.remove();
  vi.unstubAllGlobals();
});

it("keeps archived dialogue read-only and retries the explicitly selected copy with the same ID", async () => {
  const session: Session = {
    id: "archive", revision: 4, event_cursor: 3, anonymous: true, operations: [],
    document: {
      models: defaultModels, requirements: [],
      messages: [{id: "old-message", role: "user", content: "My earlier work", created_at: "2026-09-01"}],
      artifacts: ["a", "b"].map(id => ({id, kind: "test_suite", title: `Saved ${id}`, agent_prompt: "Keep original instructions.", accepted: true, blueprint: {cases: []}})),
    },
  };
  const before = structuredClone(session);
  const onContinue = vi.fn(async (_artifact: string, _request: string) => {});
  const onExport = vi.fn();
  const loadEvidence = vi.fn();
  await act(async () => root.render(<ArchivedConversation session={session} navigation={null} busy={false} error="" onContinue={onContinue} onExport={onExport} loadEvidence={loadEvidence} />));
  expect(node.textContent).toContain("read-only");
  expect(node.textContent).toContain("My earlier work");
  expect(node.querySelector("textarea,form")).toBeNull();
  expect(loadEvidence).not.toHaveBeenCalled();
  expect(onContinue).not.toHaveBeenCalled();
  const copy = [...node.querySelectorAll("button")].find(b => b.textContent === "Continue in V1")!;
  await act(async () => copy.click());
  await act(async () => copy.click());
  expect(onContinue.mock.calls[0]).toEqual(onContinue.mock.calls[1]);
  expect(onContinue).toHaveBeenCalledWith("b", expect.any(String));
  await act(async () => {
    const select = node.querySelector("select")!;
    select.value = "a";
    select.dispatchEvent(new Event("change", {bubbles: true}));
  });
  await act(async () => copy.click());
  expect(onContinue.mock.calls[2][0]).toBe("a");
  expect(onContinue.mock.calls[2][1]).not.toBe(onContinue.mock.calls[0][1]);
  await act(async () => [...node.querySelectorAll("button")].find(b => b.textContent === "Export saved work")!.click());
  expect(onExport).toHaveBeenCalledOnce();
  expect(session).toEqual(before);
});
