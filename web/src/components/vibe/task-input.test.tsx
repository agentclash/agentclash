import { draftMemory } from "@/lib/vibe-build-drafts";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { TaskMaterial } from "@/lib/vibe-inputs";
import { TaskInput } from "./task-input";
import { TaskSources } from "./task-sources";
import { vibeFetch } from "@/lib/vibe";
vi.mock("@/lib/vibe", async load => ({ ...await load<typeof import("@/lib/vibe")>(), vibeFetch: vi.fn() }));

const ready: TaskMaterial = { id: "one", session_id: "s", client_id: "c", kind: "pdf", name: "quarterly-report-with-a-very-long-filename.pdf", status: "ready", content_hash: "hash", warnings: [], pages: [{number: 1, text: "Approval is required."}] };
const selection = (material = ready) => ({ material, acknowledged: false, usage: "task_input" as const, page: 1, quote: "" });
let node: HTMLDivElement;
let root: Root;
let inputs: ReturnType<typeof useVibeInputs>;
const add = vi.fn(async () => undefined as TaskMaterial | undefined);
const list = vi.fn();
const attach = vi.fn();
const remove = vi.fn();
const configure = vi.fn();
const acknowledge = vi.fn();
const retry = vi.fn();

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.clearAllMocks(); add.mockResolvedValue(undefined); draftMemory.clear(); sessionStorage.clear();
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
  await click("Close"); await act(async () => root.unmount()); root = createRoot(node);
  await render(); await click("Add a file or text"); await click("Paste text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("Source text for the task");
  expect(add).not.toHaveBeenCalled();
  add.mockResolvedValue(ready); await click("Attach text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("");
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
  await render(true); await click("Add a file or text"); await click("Upload PDF");
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Maximum 30 pages and 10 MB");
  expect(document.querySelector('input[type="file"]')).not.toBeNull();
});

it("keeps partial acknowledgement and exact rule adoption in attachment management", async () => {
  inputs = { ...inputs, items: [selection({ ...ready, warnings: ["Page 2 could not be read."] })] };
  await render(true, true);
  expect(document.querySelector(".vibe-material-chip-status")?.textContent).toContain("Partial text");
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

it("preserves a newer pasted draft after an earlier attachment acknowledgement crosses owner unmounts", async () => {
  let finish!: (material: TaskMaterial) => void;
  add.mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  await render(false); await click("Add a file or text");
  await change(document.querySelector("textarea")!, "First submitted text"); await click("Attach text");
  await act(async () => root.unmount()); root = createRoot(node);
  await render(false); await click("Add a file or text");
  await change(document.querySelector("textarea")!, "A newer unattached draft");
  await act(async () => finish({ ...ready, kind: "text" }));
  await act(async () => root.unmount()); root = createRoot(node);
  await render(false); await click("Add a file or text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("A newer unattached draft");
});


it("keeps saved source evidence visible when another source fails and retries only that source", async () => {
  vi.mocked(vibeFetch).mockImplementation(async path => { if (String(path).endsWith("missing")) throw Error("Source expired"); return ready; });
  await act(async () => root.render(<TaskSources sessionID="s" bindings={[{input_id:"one",content_hash:"hash",usage:"task_input"},{input_id:"missing",content_hash:"hash",usage:"task_input"}]} />));
  await act(async () => { const details = node.querySelector("details")!; details.open = true; details.dispatchEvent(new Event("toggle")); });
  expect(node.textContent).toContain("Approval is required.");
  expect(node.textContent).toContain("Source expired");
  vi.mocked(vibeFetch).mockResolvedValue({...ready,id:"missing",name:"Recovered source"});
  await click("Retry source missing");
  expect(vi.mocked(vibeFetch).mock.calls.map(call => call[0])).toEqual(["/sessions/s/inputs/one","/sessions/s/inputs/missing","/sessions/s/inputs/missing"]);
  expect(node.textContent).toContain("Recovered source");
  expect(node.textContent).not.toContain("Source expired");
});

it("retains independent unattached pasted drafts for brief and trial recipients", async () => {
  inputs = {...inputs,projectID:"s",recipient:"guide"};
  await render(false); await click("Add a file or text");
  await change(document.querySelector("textarea")!, "Brief material draft");
  await click("Close");
  inputs = {...inputs,recipient:"trial:v1:thread-a"};
  await render(false); await click("Add a file or text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("");
  await change(document.querySelector("textarea")!, "Trial material draft");
  await click("Close"); inputs = {...inputs,recipient:"guide"};
  await render(false); await click("Add a file or text");
  expect((document.querySelector("textarea") as HTMLTextAreaElement).value).toBe("Brief material draft");
  expect(add).not.toHaveBeenCalled();
});

const materialToken = async () => undefined;
it("repairs named closed-composer blockers at their source fields without enabling another blocked source", async () => {
  sessionStorage.setItem("vibe-materials:s:guide", JSON.stringify([
    {id:"one",usage:"task_input",page:1,quote:"",acknowledged:false},
    {id:"policy",usage:"rules",page:1,quote:"",acknowledged:false},
  ]));
  vi.mocked(vibeFetch).mockImplementation(async path => String(path).endsWith("policy")
    ? {...ready,id:"policy",kind:"text",name:"Policy text"}
    : {...ready,warnings:["Page 2 could not be read."]});
  let owner!: ReturnType<typeof useVibeInputs>;
  function Intake() { owner=useVibeInputs("s","guide",materialToken); return <TaskInput inputs={owner} configure pdfAvailable disabled={false}/>; }
  await act(async()=>root.render(<Intake/>));
  const named = (name: string) => document.querySelector(`[aria-label="${name}"]`) as HTMLButtonElement;
  expect(node.textContent).toContain("Choose exact rule text from page 1");
  expect(node.textContent).not.toContain("Adopted rules");
  expect(owner.blocked).toBe(true);
  await act(async()=>named(`Review partial text for ${ready.name}`).click());
  const check = document.querySelector('input[type="checkbox"]') as HTMLInputElement;
  await vi.waitFor(()=>expect(document.activeElement).toBe(check));
  await act(async()=>check.click());
  expect(owner.blocked).toBe(true);
  await click("Close");
  await act(async()=>named("Choose rule text for Policy text").click());
  let quote=document.querySelector('textarea[id$="-policy-quote"]') as HTMLTextAreaElement;
  await vi.waitFor(()=>expect(document.activeElement).toBe(quote));
  expect(quote.closest("details")?.open).toBe(true);
  await change(quote,"Invented policy"); await click("Close");
  expect(node.textContent).toContain("Rule text must match page 1 exactly");
  await act(async()=>named("Repair rule text for Policy text").click());
  quote=document.querySelector('textarea[id$="-policy-quote"]') as HTMLTextAreaElement;
  await vi.waitFor(()=>expect(document.activeElement).toBe(quote));
  await change(quote,"Approval is required.");
  expect(owner.blocked).toBe(false);
  expect(owner.adoptions).toEqual([{input_id:"policy",hash:"hash",page:1,exact_quote:"Approval is required."}]);
  await click("Close");
  await act(async()=>owner.configure("policy",{quote:""}));
  await act(async()=>named("Detach Policy text and continue").click());
  expect(owner.blocked).toBe(false);
  expect(owner.items.map(item=>item.material.id)).toEqual(["one"]);
  expect(vi.mocked(vibeFetch).mock.calls.every(call=>!call[2]?.method)).toBe(true);
});

it.each(["failed", "expired"] as const)("targets the %s source replacement and detaches only on explicit confirmation", async status => {
  sessionStorage.setItem("vibe-materials:s:guide", JSON.stringify([{id:"one",usage:"task_input",page:1,quote:"",acknowledged:false}]));
  vi.mocked(vibeFetch).mockResolvedValue({...ready,status});
  let owner!: ReturnType<typeof useVibeInputs>;
  function Intake() { owner=useVibeInputs("s","guide",materialToken); return <TaskInput inputs={owner} configure pdfAvailable disabled={false}/>; }
  await act(async()=>root.render(<Intake/>));
  expect(node.querySelector(".vibe-material-blockers")?.textContent).toContain(status === "expired" ? "This material has expired" : "Reading failed");
  await act(async()=>(node.querySelector(`[aria-label="Replace material for ${ready.name}"]`) as HTMLButtonElement).click());
  await vi.waitFor(()=>expect(document.activeElement).toBe(button("Detach and paste text instead")));
  expect(owner.items).toHaveLength(1); expect(owner.blocked).toBe(true);
  await click("Detach and paste text instead");
  await vi.waitFor(()=>expect(document.activeElement).toBe(document.querySelector('textarea[id$="-paste"]')));
  expect(owner.items).toEqual([]); expect(owner.blocked).toBe(false);
  expect(vi.mocked(vibeFetch).mock.calls.every(call=>!call[2]?.method)).toBe(true);
});
