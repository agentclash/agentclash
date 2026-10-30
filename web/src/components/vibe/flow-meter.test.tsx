import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { Page } from "@playwright/test";
import { census, type Census } from "../../../e2e/vibe/flow-meter";
import { BuildStart } from "./build-start";
import { ImproveStart } from "./improve-start";

vi.mock("@playwright/test", () => ({ expect: vi.fn(), test: vi.fn() }));
let node: HTMLDivElement, root: Root;
const noop = () => {};
const measure = () => census({ evaluate: async (read: () => Census) => read() } as unknown as Page);
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  node = document.createElement("div"); document.body.append(node); root = createRoot(node);
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue({ top: 0, bottom: 44, left: 0, right: 200, width: 200, height: 44, x: 0, y: 0, toJSON: noop });
  Object.defineProperty(Element.prototype, "checkVisibility", { configurable: true, value: function (this: Element) {
    if (this.closest("[hidden]")) return false;
    for (let detail = this.closest("details"); detail; detail = detail.parentElement?.closest("details") || null) if (!detail.open && !detail.querySelector(":scope > summary")?.contains(this)) return false;
    return true;
  } });
});
afterEach(async () => { await act(async () => root.unmount()); node.remove(); Reflect.deleteProperty(Element.prototype, "checkVisibility"); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("P4: the actual census counts a roving radiogroup once, keeping other controls", async () => {
  node.innerHTML = '<div role="radiogroup" aria-label="Source"><button role="radio" tabindex="0">One</button><button role="radio" tabindex="-1">Two</button><button role="radio" tabindex="-1">Three</button></div><textarea></textarea><button>Submit</button>';
  expect((await measure()).controls.mainEnabled).toBe(3);
});
it("P4: named native radios group by name and form, while unnamed radios stay separate", async () => {
  node.innerHTML = '<form><input type="radio" name="source"><input type="radio" name="source"><input type="radio" name="other"><input type="radio"></form><form><input type="radio" name="source"></form>';
  expect((await measure()).controls.mainEnabled).toBe(4);
});
it("P4: a focusable radiogroup and its radios count once and retain enabled group state", async () => {
  node.innerHTML = '<div role="radiogroup" tabindex="0"><button role="radio" disabled>One</button><button role="radio" tabindex="0">Two</button></div><input type="radio" name="disabled" disabled><input type="radio" name="disabled" disabled>';
  expect((await measure()).controls).toEqual({ all: 2, main: 2, mainEnabled: 1 });
});
it.each([false, true])("P2/P4: census of the actual starts retains budgets (populated: %s)", async populated => {
  await act(async () => root.render(<BuildStart draft={populated ? "Answer questions about returns." : ""} onDraft={noop} price={{ kind: "trial" }} trialUsed={false} busy={false} onStart={noop} onSwitchDoor={noop} onBack={noop} />));
  // The Build budget applies before input; entering a task enables its primary.
  expect((await measure()).controls.mainEnabled).toBe(populated ? 6 : 5);
  for (const [mode, expected] of [["instructions", populated ? 8 : 7], ["replies", populated ? 6 : 5], ["pack", 4]] as const) {
    await act(async () => root.render(<ImproveStart mode={mode} onMode={noop} drafts={{ instructions: populated ? "Use my policy" : "", conversations: populated ? "Customer: Hi\nAgent: Hello" : "", rules: populated ? "Be helpful" : "" }} onDraft={noop} models={[{ id: "openai/gpt-4.1-mini", name: "openai/gpt-4.1-mini" }]} signedIn={false} busy={false} onWriteTests={noop} onSaveReplies={noop} onImportPack={noop} onOther={noop} onSwitchDoor={noop} onBack={noop} />));
    expect((await measure()).controls.mainEnabled, mode).toBe(expected);
    expect(expected).toBeLessThanOrEqual(8);
  }
});
