import { File as ByteFile } from "node:buffer";
import React, { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { BuildStart } from "./build-start";

let root: Root, node: HTMLDivElement;
const start = vi.fn(), back = vi.fn(), switchDoor = vi.fn();
type Props = React.ComponentProps<typeof BuildStart>;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(p: Partial<Props> = {}) {
  function Screen() { const [draft, onDraft] = useState(p.draft || ""); return <BuildStart draft={draft} onDraft={onDraft} price={{ kind: "trial" }} trialUsed={false} busy={false} onStart={start} onBack={back} onSwitchDoor={switchDoor} {...p} />; }
  await act(async () => root.render(<Screen />));
}
const button = (text: string) => [...node.querySelectorAll<HTMLButtonElement>("button")].find(b => b.textContent === text)!;
const submit = () => button("Build and run 3 tests");
const input = () => node.querySelector("textarea")!;
async function type(text: string) { await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input(), text); input().dispatchEvent(new Event("input", { bubbles: true })); }); }
async function send() { await act(async () => { node.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); }); }
it("places a four-line labelled input after one heading and keeps the primary filled with its disabled reason", async () => {
  await render();
  expect(node.querySelectorAll("h1")).toHaveLength(1);
  expect(node.querySelector("h1")?.textContent).toBe("What task should the agent handle?");
  expect(input().getAttribute("aria-labelledby")).toBe(node.querySelector("h1")?.id);
  expect(input().rows).toBeGreaterThanOrEqual(4);
  expect(document.activeElement).toBe(input());
  expect(submit().classList.contains("vibe-button-primary")).toBe(true);
  expect(submit().disabled).toBe(true);
  expect(node.querySelector(`#${submit().getAttribute("aria-describedby")}`)?.textContent).toContain("Describe the task");
  expect(node.querySelector(".vibe-material-hint")?.closest("form")).toBeNull();
  await type("x".repeat(19)); await send(); expect(start).not.toHaveBeenCalled();
  await type("  " + "x".repeat(20) + "  "); expect(submit().disabled).toBe(false);
  await send(); expect(start).toHaveBeenCalledWith("x".repeat(20));
});
it("fills examples without sending and calls Back and the door switch", async () => {
  await render();
  for (const text of ["Answer return questions", "Sort support emails"]) {
    await act(async () => button(text).click()); expect(input().value.length).toBeGreaterThan(20); expect(start).not.toHaveBeenCalled(); expect(document.activeElement).toBe(input());
  }
  await act(async () => button("Back").click()); expect(back).toHaveBeenCalledOnce();
  await act(async () => button("Already have an agent? Improve it instead.").click()); expect(switchDoor).toHaveBeenCalledOnce();
});
it.each([[{ kind: "trial" }, "Free trial"], [{ kind: "ceiling", nanoUSD: 500000000 }, "Up to $0.50"], [{ kind: "unknown" }, "Price available when you start"]] as const)("shows the supplied price %j without inventing a duration", async (price, label) => {
  await render({ price }); expect(node.querySelector(".vibe-build-price")?.textContent).toBe(label); expect(node.textContent).not.toMatch(/1.2 min|usually|about/);
});
it.each([{ trialUsed: true }, { busy: true }, { blockedReason: "No models are available." }])("blocks submission and keeps the draft for %j", async extra => {
  await render({ draft: "Answer questions using our return policy.", ...extra });
  expect(submit().disabled).toBe(true); await send(); expect(start).not.toHaveBeenCalled(); expect(input().value).toContain("return policy");
  if (extra.trialUsed) { expect(node.textContent).toContain("Your free build is used."); expect(node.querySelector("a")?.textContent).toBe("Sign in to continue."); }
  if (extra.blockedReason) expect(node.textContent).toContain(extra.blockedReason);
});
it("shows errors under the primary without clearing input", async () => {
  await render({ draft: "Answer questions using our return policy.", error: "Couldn’t start. Try again." });
  const alert = node.querySelector('[role="alert"]')!;
  expect(alert.textContent).toContain("Try again"); expect(submit().compareDocumentPosition(alert) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy(); expect(input().value).toContain("return policy");
});
it("attaches text to the draft without starting and reports unreadable or unsupported files", async () => {
  await render();
  const fileInput = node.querySelector<HTMLInputElement>('input[type="file"]')!;
  const choose = async (file: Partial<File>) => { Object.defineProperty(fileInput, "files", { configurable: true, value: [file] }); await act(async () => fileInput.dispatchEvent(new Event("change", { bubbles: true }))); };
  const click = vi.spyOn(fileInput, "click"); await act(async () => button("Attach a file").click()); expect(click).toHaveBeenCalledOnce();
  await choose({ name: "policy.txt", size: 30, arrayBuffer: async () => new TextEncoder().encode("Use a 30-day return policy.").buffer });
  expect(input().value).toBe("Use a 30-day return policy."); expect(start).not.toHaveBeenCalled();
  await choose({ name: "photo.png", size: 20 }); expect(node.querySelector('[role="alert"]')?.textContent).toContain("Choose a text file");
  await choose({ name: "broken.txt", size: 20, arrayBuffer: async () => { throw new Error("Unreadable"); } }); expect(node.querySelector('[role="alert"]')?.textContent).toContain("Couldn’t read"); expect(input().value).toBe("Use a 30-day return policy.");
});


it.each([
  ["UTF-16LE", [0xff, 0xfe, 0x41, 0x00, 0x42, 0x00]],
  ["Windows-1252", [0x43, 0x61, 0x66, 0xe9]],
] as const)("rejects %s attachment bytes without changing the draft", async (_encoding, bytes) => {
  await render(); await type("Keep this original task description.");
  const fileInput = node.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new ByteFile([new Uint8Array(bytes)], "policy.txt", { type: "text/plain" });
  Object.defineProperty(fileInput, "files", { configurable: true, value: [file] });
  await act(async () => fileInput.dispatchEvent(new Event("change", { bubbles: true })));
  expect(node.querySelector('[role="alert"]')?.textContent).toBe("This file is not UTF-8 text. Save it as UTF-8 and try again.");
  expect(input().value).toBe("Keep this original task description.");
  expect(start).not.toHaveBeenCalled();
  expect(submit().disabled).toBe(false);
});

it("accepts UTF-8 bytes with a BOM and appends the decoded text", async () => {
  await render(); await type("Original task.");
  const fileInput = node.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new ByteFile([new Uint8Array([0xef, 0xbb, 0xbf]), "Café returns policy."], "policy.txt");
  Object.defineProperty(fileInput, "files", { configurable: true, value: [file] });
  await act(async () => fileInput.dispatchEvent(new Event("change", { bubbles: true })));
  expect(input().value).toBe("Original task.\n\nCafé returns policy.");
  expect(node.querySelector('[role="alert"]')).toBeNull();
});
