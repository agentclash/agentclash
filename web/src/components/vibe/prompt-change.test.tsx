import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CopyNewInstructions } from "./prompt-change";
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
