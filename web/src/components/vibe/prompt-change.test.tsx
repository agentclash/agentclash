import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PromptChange, CopyNewInstructions } from "./prompt-change";
let node: HTMLDivElement, root: Root;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); node = document.createElement("div"); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); vi.unstubAllGlobals(); });
it("copies the exact new instructions after comparison and explains how to use them", async () => {
  const copy = vi.fn().mockResolvedValue(undefined); Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: copy } });
  await act(async () => root.render(<CopyNewInstructions after="Exact new policy" />));
  expect(node.textContent).toContain("Paste them into your app. We don't change your live app.");
  expect(node.querySelector("button")!.textContent).toBe("Copy the new instructions");
  await act(async () => node.querySelector("button")!.click()); expect(copy).toHaveBeenCalledWith("Exact new policy");
});
it("offers selectable instructions when the clipboard is unavailable", async () => {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockRejectedValue(Error("Denied")) } });
  await act(async () => root.render(<CopyNewInstructions after="Exact policy" />));
  await act(async () => node.querySelector("button")!.click());
  expect(node.querySelector("textarea")?.value).toBe("Exact policy"); expect(node.querySelector('[role="status"]')).not.toBeNull();
});

it("does not retain a copied label for different instructions", async () => {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
  await act(async () => root.render(<CopyNewInstructions after="First policy" />));
  await act(async () => node.querySelector("button")!.click());
  await act(async () => root.render(<CopyNewInstructions after="Second policy" />));
  expect(node.querySelector("button")?.textContent).toBe("Copy the new instructions");
});


it("renders the proposed card with versions, line markers and accessible changes", async () => {
  await act(async () => root.render(<PromptChange before={"Keep\nOld"} after={"Keep\nNew"} beforeVersion={1} afterVersion={2} defaultOpen />));
  const card = node.querySelector("details")!;
  expect(card.open).toBe(true);
  expect(card.querySelector(":scope > summary")?.textContent).toContain("Proposed change");
  expect(card.querySelector(":scope > summary")?.textContent).toContain("v1 → v2");
  const diff = node.querySelector('[aria-label="Instruction changes"]')!;
  expect(diff.querySelector("del")?.textContent).toBe("Removed: Old");
  expect(diff.querySelector("ins")?.textContent).toBe("Added: New");
  expect([...diff.querySelectorAll('[aria-hidden="true"]')].map(n => n.textContent)).toEqual(["", "−", "+"]);
  expect(diff.querySelector("del .sr-only")?.textContent).toBe("Removed: ");
  expect(diff.querySelector("ins .sr-only")?.textContent).toBe("Added: ");
});

it("supports a saved title and omits incomplete versions and absent actions", async () => {
  await act(async () => root.render(<PromptChange title="Instruction change" before="Old" after="New" beforeVersion={1} />));
  expect(node.querySelector("details")?.open).toBe(false);
  expect(node.querySelector("summary")?.textContent).toBe("Instruction change");
  expect(node.querySelector("footer")).toBeNull();
});

it("places working supplied actions in the card footer", async () => {
  let runs = 0;
  await act(async () => root.render(<PromptChange before="Old" after="New" defaultOpen actions={<button onClick={() => runs++}>Apply and run 3 tests</button>} />));
  const action = node.querySelector(".vibe-prompt-change > footer button") as HTMLButtonElement;
  expect(action.textContent).toBe("Apply and run 3 tests");
  await act(async () => action.click());
  expect(runs).toBe(1);
});

it("keeps the run action visible and operable after closing the disclosure", async () => {
  let runs = 0;
  await act(async () => root.render(<PromptChange before="Old" after="New" defaultOpen actions={<button onClick={() => runs++}>Apply and run 3 tests</button>} />));
  const disclosure = node.querySelector("details")!;
  await act(async () => disclosure.querySelector("summary")!.click());
  expect(disclosure.open).toBe(false);
  const action = node.querySelector("footer button") as HTMLButtonElement;
  expect(action).not.toBeNull();
  expect(action.closest("details:not([open])")).toBeNull();
  expect(action.closest("[hidden]")).toBeNull();
  expect(getComputedStyle(action).display).not.toBe("none");
  expect(getComputedStyle(action).visibility).toBe("visible");
  expect(action.disabled).toBe(false);
  await act(async () => action.click());
  expect(runs).toBe(1);
});

it("renders large unchanged instructions with folded context", async () => {
  const instructions = "a\n" + "\n".repeat(65530) + "b";
  await act(async () => root.render(<PromptChange before={instructions} after={instructions} defaultOpen />));
  expect(node.querySelector(".vibe-diff-fold")?.textContent).toBe("65526 unchanged lines");
  expect(node.querySelectorAll(".vibe-diff-line")).toHaveLength(6);
  expect(node.querySelector("pre")?.textContent).toBe(instructions);
});

it("folds leading, middle and trailing context to three lines around changes", async () => {
  const lines = Array.from({ length: 24 }, (_, i) => `Line ${i + 1}`);
  const changed = [...lines]; changed[6] = "First change"; changed[17] = "Second change";
  await act(async () => root.render(<PromptChange before={lines.join("\n")} after={changed.join("\n")} defaultOpen />));
  const diff = node.querySelector('[aria-label="Instruction changes"]')!;
  expect(diff.textContent).toContain("3 unchanged lines");
  expect(diff.textContent).toContain("4 unchanged lines");
  expect([...diff.querySelectorAll(".vibe-diff-line-text")].some(n => n.textContent === "Line 1")).toBe(false);
  expect(diff.textContent).not.toContain("Line 24");
  expect(diff.textContent).toContain("Line 4");
  expect(diff.textContent).toContain("Line 21");
  expect(node.querySelector("pre")?.textContent).toBe(changed.join("\n"));
});

it("keeps short unchanged instructions visible without changed markers", async () => {
  await act(async () => root.render(<PromptChange before={"Same\nPolicy"} after={"Same\nPolicy"} defaultOpen />));
  const diff = node.querySelector('[aria-label="Instruction changes"]')!;
  expect(diff.textContent).toBe("SamePolicy");
  expect(diff.querySelector("ins, del")).toBeNull();
});
