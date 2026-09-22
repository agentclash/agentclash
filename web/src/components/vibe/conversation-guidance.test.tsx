import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ConversationGuidance } from "./conversation-guidance";
import { primarySurface, readExampleCard } from "@/lib/vibe-presentation";

const scope = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const origin = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const card = { id: origin, scope_id: scope, origin_message_id: origin, kind: "example", input: "PDF heading: Returns", expected: "# Returns", illustrative: true };
let node: HTMLDivElement; let root: Root;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); node = document.createElement("div"); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); vi.unstubAllGlobals(); });

it("shows an explicitly illustrative comparison, with a reversible local hide and no actions", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  await act(async () => root.render(<ConversationGuidance cards={[card]} scope={scope} message={origin} />));
  expect(node.textContent).toContain("Example only"); expect(node.textContent).toContain("Nothing was run");
  expect(node.textContent).toContain("# Returns");
  await act(async () => node.querySelector("button")!.click());
  expect(node.textContent).not.toContain("# Returns");
  await act(async () => node.querySelector("button")!.click());
  expect(node.textContent).toContain("# Returns"); expect(fetch).not.toHaveBeenCalled();
});
it.each([
  { ...card, scope_id: origin }, { ...card, origin_message_id: scope }, { ...card, kind: "html" },
  { ...card, url: "javascript:alert(1)" }, { ...card, illustrative: false }, { ...card, expected: "" },
  { ...card, expected: "x".repeat(1601) },
])("ignores malformed, invented or differently scoped cards", async value => {
  expect(readExampleCard(value, scope, origin)).toBeNull();
  await act(async () => root.render(<ConversationGuidance cards={[value]} scope={scope} message={origin} />));
  expect(node.textContent).toBe("");
});
it("renders HTML, links and fake run instructions as inert example text", async () => {
  const literal = '<img src=x onerror=fetch("/save")> [Run](javascript:alert(1))';
  await act(async () => root.render(<ConversationGuidance cards={[{ ...card, input: literal }]} scope={scope} message={origin} />));
  expect(node.textContent).toContain(literal); expect(node.querySelector("img,a,script")).toBeNull();
});

it("gives one surface priority from actual state and explicit typing, without inferred expertise", () => {
  const state = { busy: false, dirty: false, typing: false, choice: false, results: false, tests: false, recovery: false };
  expect(primarySurface(state)).toBe("composer");
  expect(primarySurface({ ...state, tests: true })).toBe("tests");
  expect(primarySurface({ ...state, tests: true, choice: true })).toBe("choices");
  expect(primarySurface({ ...state, choice: true, results: true })).toBe("results");
  expect(primarySurface({ ...state, results: true, recovery: true })).toBe("recovery");
  expect(primarySurface({ ...state, results: true, choice: true, typing: true })).toBe("composer");
  expect(primarySurface({ ...state, results: true, typing: true, dirty: true })).toBe("tests");
  expect(primarySurface({ ...state, dirty: true, busy: true })).toBe("none");
});
