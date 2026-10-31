import { readFileSync } from "node:fs";
import { File as ByteFile } from "node:buffer";
import React, { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { BuildStart } from "./build-start";

let root: Root, node: HTMLDivElement;
const start = vi.fn(), back = vi.fn(), switchDoor = vi.fn();
type Props = React.ComponentProps<typeof BuildStart>;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); node = document.createElement("div"); node.className = "vibe-workspace"; document.body.append(node); root = createRoot(node); });
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
  expect(input().labels?.[0]?.textContent).toBe("What task should the agent handle?");
  expect(input().getAttribute("aria-labelledby")).toBeNull();
  expect(input().hasAttribute("aria-label")).toBe(false);
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

it("sends Enter once and ignores another Enter or form submit while busy", async () => {
  function Screen() {
    const [busy, setBusy] = useState(false);
    return <BuildStart draft="Answer questions using our return policy." onDraft={() => {}} price={{ kind: "trial" }} trialUsed={false} busy={busy} onStart={text => { start(text); setBusy(true); }} onBack={back} onSwitchDoor={switchDoor} />;
  }
  await act(async () => root.render(<Screen />));
  const enter = async (extra: KeyboardEventInit = {}) => {
    const event = new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true, ...extra });
    await act(async () => { input().dispatchEvent(event); });
    return event;
  };
  await enter({ shiftKey: true }); await enter({ isComposing: true });
  expect(start).not.toHaveBeenCalled();
  expect((await enter()).defaultPrevented).toBe(true);
  expect(start).toHaveBeenCalledExactlyOnceWith("Answer questions using our return policy.");
  expect(submit().disabled).toBe(true);
  expect(submit().getAttribute("aria-busy")).toBe("true");
  await enter(); await send();
  expect(start).toHaveBeenCalledOnce();
  expect(input().value).toBe("Answer questions using our return policy.");
});


it("P2: exposes attachment after two chips and the switch at the bottom without a disclosure", async () => {
  await render();
  expect(node.querySelector("details")).toBeNull();
  const row = node.querySelector(".vibe-build-example-chips")!;
  expect([...row.querySelectorAll("button")].map(control => control.textContent)).toEqual(["Answer return questions", "Sort support emails", "Attach a file"]);
  expect(button("Attach a file").querySelector("svg.lucide-paperclip")).not.toBeNull();
  const switcher = button("Already have an agent? Improve it instead.");
  expect(switcher.parentElement).toBe(node.querySelector("section"));
  expect(node.querySelector("form")!.compareDocumentPosition(switcher) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(node.querySelector(".vibe-material-hint")?.textContent).toContain("Leave out secrets and customer IDs.");
  const dataUse = node.querySelector('.vibe-material-hint a[href="/docs/vibe-evals/data-use"]');
  expect(dataUse?.textContent).toContain("Data use");
  const controls = [...node.querySelectorAll<HTMLButtonElement>("button, textarea, select, a[href], summary")].filter(control => !control.closest("header") && !control.disabled);
  expect(controls).toHaveLength(6);
});
it("P3: puts a left-aligned primary and price immediately below chips with its reason beneath", async () => {
  const style = document.createElement("style"); style.textContent = readFileSync("src/components/vibe/build-output.css", "utf8"); document.head.append(style);
  try {
    await render();
    const actions = node.querySelector(".vibe-build-start-actions")!;
    expect(node.querySelector(".vibe-build-example-chips")!.nextElementSibling).toBe(actions);
    expect(actions.querySelector(".vibe-build-start-submit")!.firstElementChild).toBe(submit());
    expect(submit().nextElementSibling?.textContent).toBe("Free trial");
    expect(getComputedStyle(actions).justifyContent).toBe("flex-start");
    expect(parseFloat(getComputedStyle(submit().parentElement!).marginInlineStart)).toBe(0);
    const reason = document.getElementById(submit().getAttribute("aria-describedby")!)!;
    expect(actions.nextElementSibling).toBe(reason);
    expect(reason.textContent).toBe("Describe the task in a sentence or two.");
  } finally { style.remove(); }
});
it("associates each mounted Build question with its own input", async () => {
  await render();
  expect(input().id).not.toBe("");
  await act(async () => root.render(<><BuildStart draft="" onDraft={() => {}} price={{ kind: "trial" }} trialUsed={false} busy={false} onStart={start} onBack={back} onSwitchDoor={switchDoor} /><BuildStart draft="" onDraft={() => {}} price={{ kind: "trial" }} trialUsed={false} busy={false} onStart={start} onBack={back} onSwitchDoor={switchDoor} /></>));
  const inputs = [...node.querySelectorAll("textarea")]; expect(new Set(inputs.map(area => area.id)).size).toBe(2);
  for (const area of inputs) expect(area.labels?.[0]?.htmlFor).toBe(area.id);
});

it("uses the display serif and a flat card with a distinct keyboard focus ring", async () => {
  const style = document.createElement("style"); style.textContent = readFileSync("src/components/vibe/build-output.css", "utf8") + readFileSync("src/components/vibe/project-workspace.css", "utf8"); document.head.append(style);
  try {
    await render();
    expect(getComputedStyle(node.querySelector("h1")!).fontFamily).toBe("var(--vibe-font-serif)");
    expect(getComputedStyle(node.querySelector("h1")!).fontWeight).toBe("400");
    expect(getComputedStyle(node.querySelector("form")!).boxShadow).toBe("none");
    expect(style.textContent).toContain("outline: 2px solid var(--vibe-focus)");
  } finally { style.remove(); }
});

it("PR7 puts a file error beside Attach a file and associates a start error with the task", async () => {
  await render({ draft: "Keep this task description.", error: "Cannot start this task." });
  expect(document.getElementById(input().getAttribute("aria-describedby")!.split(" ").at(-1)!)?.textContent).toBe("Cannot start this task.");
  const file = node.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(file, "files", { configurable: true, value: [{ name: "photo.png", size: 20 }] });
  await act(async () => file.dispatchEvent(new Event("change", { bubbles: true })));
  const attach = button("Attach a file");
  const error = document.getElementById(attach.getAttribute("aria-describedby")!);
  expect(error?.textContent).toContain("Choose a text file");
  expect(error?.previousElementSibling?.contains(attach)).toBe(true);
  expect(node.querySelectorAll('[role="alert"]')).toHaveLength(2);
});
