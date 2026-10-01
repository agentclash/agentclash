import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { TaskOutput } from "./task-output";

let root: Root, node: HTMLDivElement;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
async function render(text: string) { await act(async () => root.render(<TaskOutput text={text} />)); }
function button(name: string) { return Array.from(node.querySelectorAll("button")).find(b => b.textContent === name)!; }

it("offers JSON only for one complete document and keeps the original copy", async () => {
  const original = '```json\n{"id":900719925474099312345}\n```';
  const writeText = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
  await render(original);
  expect(Array.from(node.querySelectorAll(".vibe-output-download button")).map(o => o.textContent)).toEqual(["Markdown (.md)", "JSON (.json)"]);
  await act(async () => button("Copy output").click());
  expect(writeText).toHaveBeenCalledWith(original);
  await render(`Result:\n${original}`);
  expect(Array.from(node.querySelectorAll(".vibe-output-download button")).map(o => o.textContent)).toEqual(["Markdown (.md)"]);
});

it("keeps long output fully copyable and gives an explicit expansion control", async () => {
  const original = "Long answer.\n".repeat(1200);
  const writeText = vi.fn().mockRejectedValue(new Error("denied"));
  vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
  vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(900);
  await render(original);
  expect(node.querySelector(".vibe-output-preview")?.hasAttribute("data-collapsed")).toBe(true);
  expect(node.querySelector(".vibe-output-content")?.textContent).toBe(original.trimEnd());
  await act(async () => button("Copy output").click());
  expect(writeText).toHaveBeenCalledWith(original);
  expect(node.querySelector("[role=status]")?.textContent).toContain("Select the output");
  await act(async () => button("Show full output").click());
  expect(node.querySelector(".vibe-output-preview")?.hasAttribute("data-collapsed")).toBe(false);
  expect(node.querySelector(".vibe-output-content")?.textContent).toBe(original.trimEnd());
});
