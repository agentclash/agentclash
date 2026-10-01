import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { TaskMaterial } from "@/lib/vibe-inputs";
import { TaskInput } from "./task-input";

const ready: TaskMaterial = { id: "one", session_id: "s", client_id: "c", kind: "pdf", name: "quarterly-report-with-a-very-long-filename.pdf", status: "ready", content_hash: "hash", warnings: [], pages: [{number: 1, text: "Approval is required."}] };
const selection = (material = ready) => ({ material, acknowledged: false, usage: "task_input" as const, page: 1, quote: "" });
let node: HTMLDivElement;
let root: Root;
let inputs: ReturnType<typeof useVibeInputs>;
const add = vi.fn();
const list = vi.fn();
const attach = vi.fn();
const remove = vi.fn();
const configure = vi.fn();
const acknowledge = vi.fn();
const retry = vi.fn();

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.clearAllMocks();
  node = document.createElement("div"); document.body.append(node); root = createRoot(node);
  inputs = { items: [], busy: false, error: "", canRetry: false, blocked: false, bindings: [], adoptions: [], add, list, attach, remove, configure, acknowledge, retry } as unknown as ReturnType<typeof useVibeInputs>;
});
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(pdfAvailable = true, configureUse = false) { await act(async () => root.render(<TaskInput inputs={inputs} pdfAvailable={pdfAvailable} disabled={false} configure={configureUse} />)); }
function button(name: string) { const b = Array.from(document.querySelectorAll("button")).find(button => button.textContent?.trim() === name); if (!b) throw Error(`Missing button ${name}`); return b as HTMLButtonElement; }
async function click(name: string) { await act(async () => button(name).click()); }
async function change(element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(element.tagName === "TEXTAREA" ? HTMLTextAreaElement.prototype : element.tagName === "SELECT" ? HTMLSelectElement.prototype : HTMLInputElement.prototype, "value")!;
  await act(async () => { descriptor.set!.call(element, value); element.dispatchEvent(new Event("input", { bubbles: true })); element.dispatchEvent(new Event("change", { bubbles: true })); });
}

it("opening and closing the dialog sends nothing and restores focus", async () => {
  await render();
  await click("Add a file or text");
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  expect(document.activeElement?.textContent).toBe("Add material");
  expect(add).not.toHaveBeenCalled();
  await click("Close");
  expect(add).not.toHaveBeenCalled();
  expect(document.activeElement).toBe(button("Add a file or text"));
});

it("keeps pasted draft after close and uses one explicit attach action", async () => {
  await render(); await click("Add a file or text"); await click("Paste text");
  const area = document.querySelector("textarea")!;
  await change(area, "Source text for the task");
  await click("Close"); await click("Add a file or text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("Source text for the task");
  expect(add).not.toHaveBeenCalled();
  await click("Attach text");
  expect(add).toHaveBeenCalledTimes(1);
  expect(add).toHaveBeenCalledWith("Source text for the task");
});

it("shows PDF limits and availability, and keeps retry visible while the dialog is closed", async () => {
  inputs = { ...inputs, error: "Connection lost", canRetry: true };
  await render(false); await click("Add a file or text");
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("PDF reading isn’t available");
  await click("Close");
  await click("Retry upload");
  expect(retry).toHaveBeenCalledTimes(1);
  await render(true); await click("Add a file or text");
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Maximum 30 pages and 10 MB");
  expect(document.querySelector('input[type="file"]')).not.toBeNull();
});

it("keeps partial acknowledgement and exact rule adoption in attachment management", async () => {
  inputs = { ...inputs, items: [selection({ ...ready, warnings: ["Page 2 could not be read."] })] };
  await render(true, true);
  expect(document.querySelector(".vibe-material-chip-status")?.textContent).toBe("Partial text");
  await act(async () => (document.querySelector('[aria-label^="Manage attachment"]') as HTMLButtonElement).click());
  const check = document.querySelector('input[type="checkbox"]') as HTMLInputElement;
  await act(async () => { check.click(); });
  expect(acknowledge).toHaveBeenCalledWith("one", true);
  await act(async () => (document.querySelector(".vibe-material-config summary") as HTMLElement).click());
  const select = document.querySelector(".vibe-material-config select") as HTMLSelectElement;
  await change(select, "rules");
  expect(configure).toHaveBeenCalledWith("one", { usage: "rules" });
  inputs = { ...inputs, items: [{...inputs.items[0], usage: "rules", quote: "wrong"}] };
  await render(true, true);
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Copy the text exactly from this page.");
  await click("Detach");
  expect(remove).toHaveBeenCalledWith("one");
  await click("Delete file"); await click("Delete file permanently");
  expect(remove).toHaveBeenCalledWith("one", true);
});

it("loads saved material only after opening its disclosure", async () => {
  list.mockResolvedValue([{ ...ready, id: "saved" }]);
  await render(); await click("Add a file or text");
  expect(list).not.toHaveBeenCalled();
  await act(async () => (document.querySelector(".vibe-material-saved summary") as HTMLElement).click());
  expect(list).toHaveBeenCalledTimes(1);
  expect(document.querySelector('[aria-label="Saved material"]')?.textContent).toContain(ready.name);
  await click("Attach");
  expect(attach).toHaveBeenCalledWith("saved");
});
