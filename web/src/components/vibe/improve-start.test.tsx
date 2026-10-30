import { readFileSync } from "node:fs";
import React, { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ImproveStart } from "./improve-start";

let node: HTMLDivElement; let root: Root;
const write = vi.fn(), replies = vi.fn(), pack = vi.fn(), other = vi.fn(), back = vi.fn(), switchDoor = vi.fn();
type Props = React.ComponentProps<typeof ImproveStart>;
const initial = { instructions: "", model: "model-a", conversations: "", rules: "" };
function Harness(p: Partial<Props>) {
  const [mode, onMode] = useState<Props["mode"]>(p.mode || "instructions");
  const [drafts, setDrafts] = useState(initial);
  return <ImproveStart {...{ mode, onMode, drafts, onDraft: patch => setDrafts(old => ({ ...old, ...patch })), models: [{ id: "model-a", name: "Model A" }, { id: "model-b", name: "Model B" }], signedIn: false, busy: false, onWriteTests: write, onSaveReplies: replies, onImportPack: pack, onOther: other, onBack: back, onSwitchDoor: switchDoor, ...p }} />;
}
beforeEach(() => {
  vi.clearAllMocks(); vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  node = document.createElement("div"); node.className = "vibe-workspace"; document.body.append(node); root = createRoot(node);
});
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
const draw = async (p: Partial<Props> = {}) => act(async () => root.render(<Harness {...p} />));
const button = (name: string) => [...node.querySelectorAll<HTMLButtonElement>("button")].find(b => b.textContent?.trim() === name)!;
async function click(name: string) { await act(async () => button(name).click()); }
async function type(label: string, value: string) {
  const area = [...node.querySelectorAll<HTMLTextAreaElement>("textarea")].find(a => a.labels?.[0]?.textContent?.startsWith(label))!;
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(area, value); area.dispatchEvent(new Event("input", { bubbles: true })); });
}
it("shows one heading and the default labelled instructions with sample, model and source request", async () => {
  await draw(); expect(node.querySelectorAll("h1")).toHaveLength(1); expect(document.activeElement).toBe(node.querySelector("h1"));
  expect(node.querySelector("h1")?.textContent).toBe("How do you want to test your agent?");
  expect(node.querySelector('[role="radiogroup"]')).not.toBeNull();
  expect(button("Instructions").getAttribute("aria-checked")).toBe("true");
  expect(button("Write 3 tests").disabled).toBe(true);
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  await click("Use sample instructions");
  expect(node.querySelector("textarea")!.value).toContain("returns");
  const model = node.querySelector("select")!;
  expect(model.labels?.[0]?.textContent).toContain("Which model does your agent use?");
  await act(async () => { model.value = "model-b"; model.dispatchEvent(new Event("change", { bubbles: true })); });
  await click("Write 3 tests"); expect(write).toHaveBeenCalledWith({ text: expect.stringContaining("returns"), model: "model-b" });
  const help = [...node.querySelectorAll("details")].find(d => d.querySelector("summary")?.textContent === "Where do I find these?")!;
  expect(help.textContent).toContain("Do not rewrite them.");
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
  await click("Copy request"); expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining("exact instructions"));
});
it("keeps each draft through keyboard radio navigation and counts plain text and JSON replies", async () => {
  await draw(); await type("Agent instructions", "Keep my policy");
  await act(async () => { button("Instructions").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true })); });
  expect(document.activeElement).toBe(button("Real replies"));
  expect(button("Real replies").getAttribute("tabindex")).toBe("0");
  const area = node.querySelector("textarea")!; expect(area.placeholder).toContain("Customer:"); expect(area.placeholder).toContain("---");
  await type("Conversations", "Customer: One\nAgent: Two\n---\nCustomer: Three\nAgent: Four");
  await type("What a good reply must do", "Never promise refunds");
  expect(node.textContent).toContain("2 conversations · 4 messages");
  await click("Save and check"); expect(replies).toHaveBeenCalledWith({ conversations: area.value, rules: "Never promise refunds" });
  await click("Instructions"); expect(node.querySelector("textarea")!.value).toBe("Keep my policy");
  await click("Real replies"); expect(node.querySelectorAll("textarea")[1].value).toBe("Never promise refunds");
  await type("Conversations", JSON.stringify({ conversations: [{ messages: [{ role: "user", content: "Hi" }, { role: "assistant", content: "Hello" }] }] }));
  expect(node.textContent).toContain("1 conversation · 2 messages");
  await act(async () => { button("Real replies").dispatchEvent(new KeyboardEvent("keydown", { key: "End", bubbles: true })); });
  expect(document.activeElement).toBe(button("Test pack"));
  await act(async () => { button("Test pack").dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true })); });
  expect(document.activeElement).toBe(button("Instructions"));
});
it("keeps errors by the primary, blocks submission and preserves the input", async () => {
  await draw(); await type("Agent instructions", "Saved draft");
  await draw({ blockedReason: "Inference unavailable", error: "Could not create project" });
  expect(button("Write 3 tests").disabled).toBe(true);
  expect(node.querySelector('[role="alert"]')?.textContent).toContain("Could not create project");
  expect(node.textContent).toContain("Inference unavailable");
  await act(async () => node.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(write).not.toHaveBeenCalled(); expect(node.querySelector("textarea")!.value).toBe("Saved draft");
  await draw({ busy: true }); expect(button("Write 3 tests").disabled).toBe(true);
});
it("puts import errors in the drop zone and supports choosing and dropping a file", async () => {
  await draw({ mode: "pack", error: "Invalid test pack" });
  const zone = node.querySelector('[aria-label="Test pack file"]')!;
  expect(zone.querySelector('[role="alert"]')?.textContent).toBe("Invalid test pack");
  const input = node.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["{}"], "tests.json", { type: "application/json" });
  Object.defineProperty(input, "files", { configurable: true, value: [file] });
  await act(async () => input.dispatchEvent(new Event("change", { bubbles: true }))); expect(pack).toHaveBeenCalledWith(file);
  const drop = new Event("drop", { bubbles: true, cancelable: true }); Object.defineProperty(drop, "dataTransfer", { value: { files: [file] } });
  await act(async () => zone.dispatchEvent(drop)); expect(pack).toHaveBeenCalledTimes(2);
  await draw({ mode: "pack", busy: true }); await act(async () => zone.dispatchEvent(drop)); expect(pack).toHaveBeenCalledTimes(2);
});
it("explains guest advanced paths and keeps Back and door switch reachable", async () => {
  await draw(); expect(node.textContent).toContain("Sign in to connect a live app"); expect(node.textContent).toContain("Sign in to import an SDK report");
  expect(button("Connect a live app")).toBeUndefined();
  expect(node.querySelector("a")?.getAttribute("href")).toBe("/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dimprove");
  await click("Describe it in chat"); expect(other).toHaveBeenCalledWith("chat");
  await click("Back"); expect(back).toHaveBeenCalledOnce();
  await click("Build an agent instead"); expect(switchDoor).toHaveBeenCalledOnce();
  await draw({ signedIn: true }); await click("Connect a live app"); await click("Import an SDK report");
  expect(other.mock.calls.slice(-2)).toEqual([["live_app"], ["sdk_report"]]);
});


it("P4: exposes the sample and friendly model while keeping only rare choices in disclosures", async () => {
  await draw({ drafts: { instructions: "Keep the policy", model: "model-a", conversations: "Customer: Hi\nAgent: Hello", rules: "Be helpful" } });
  expect(button("Back").closest("header")).not.toBeNull();
  expect(button("Use sample instructions").closest("details")).toBeNull();
  expect(button("Build an agent instead").closest("details")?.querySelector("summary")?.textContent).toBe("Other ways to bring your agent");
  expect(node.querySelector("select")?.closest("details")).toBeNull();
  expect([...node.querySelectorAll("option")].map(option => option.textContent)).toContain("Model A");
  const counts: number[] = [];
  for (const mode of ["Instructions", "Real replies", "Test pack"]) {
    await click(mode);
    const controls = [...node.querySelectorAll<HTMLElement>("button, textarea, select, a[href], summary")].filter(control => {
      if (control.closest("header") || (control as HTMLButtonElement).disabled) return false;
      for (let detail = control.parentElement?.closest("details"); detail; detail = detail.parentElement?.closest("details")) if (!detail.open && !detail.querySelector(":scope > summary")?.contains(control)) return false;
      return true;
    });
    expect(node.querySelectorAll('[role="radio"][tabindex="0"]')).toHaveLength(1);
    if (mode !== "Instructions") { expect(button("Use sample instructions")).toBeUndefined(); expect(node.querySelector("select")).toBeNull(); }
    counts.push(controls.length - 2);
  }
  expect(counts).toEqual([8, 6, 4]);
});
it("labels every Improve input and gives independent mounts unique field ids", async () => {
  await draw();
  const first = node.querySelector("textarea")!.id;
  expect(first).not.toBe(""); expect(first).not.toBe("vibe-instructions-source");
  for (const mode of ["Instructions", "Real replies", "Test pack"]) {
    await click(mode);
    for (const control of node.querySelectorAll<HTMLTextAreaElement | HTMLSelectElement | HTMLInputElement>("textarea, select, input")) {
      expect(control.id).not.toBe(""); expect(control.labels?.length).toBe(1);
      expect(control.labels?.[0]?.textContent?.trim()).not.toBe("");
    }
  }
  await act(async () => root.render(<><Harness key="first" /><Harness key="second" /></>));
  const areas = [...node.querySelectorAll("textarea")]; expect(new Set(areas.map(area => area.id)).size).toBe(2);
});

it("uses the display serif question and visible borders for every segment", async () => {
  const style = document.createElement("style"); style.textContent = (readFileSync("src/components/vibe/improve.css", "utf8") + readFileSync("src/components/vibe/project-workspace.css", "utf8")).replaceAll("var(--vibe-control-border)", "#765820"); document.head.append(style);
  try {
    await draw();
    expect(getComputedStyle(node.querySelector("h1")!).fontFamily).toBe("var(--vibe-font-serif)");
    expect(getComputedStyle(node.querySelector("h1")!).fontWeight).toBe("400");
    for (const radio of node.querySelectorAll('[role="radio"]')) {
      expect(getComputedStyle(radio).borderTopWidth).toBe("1px");
      expect(getComputedStyle(radio).borderTopStyle).toBe("solid");
    }
  } finally { style.remove(); }
});
