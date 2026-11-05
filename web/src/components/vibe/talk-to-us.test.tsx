import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { TalkToUs } from "./talk-to-us";

let node: HTMLDivElement, root: Root;
const onOpenChange = vi.fn();
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); onOpenChange.mockReset(); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(email = "hello@tryagentclash.com") {
  await act(async () => root.render(<TalkToUs open onOpenChange={onOpenChange} email={email} />));
}
const popup = () => document.querySelector('[role="dialog"]') as HTMLElement;
const send = () => [...popup().querySelectorAll("a")].find(a => a.textContent === "Email our team")!;
async function type(label: string, value: string) {
  const field = [...popup().querySelectorAll("label")].find(l => l.textContent === label)!;
  const input = document.getElementById(field.htmlFor) as HTMLTextAreaElement | HTMLInputElement;
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), "value")!.set!;
  await act(async () => { setter.call(input, value); input.dispatchEvent(new Event("input", { bubbles: true })); });
}

it("asks one question and writes the email in the visitor's own mail app", async () => {
  await render();
  expect(popup().querySelector("h2")?.textContent).toBe("What takes your team the most time?");
  expect(send().getAttribute("href")).toBe("mailto:hello%40tryagentclash.com?subject=Agents%20for%20our%20team");
  await type("What work should agents take on?", "We answer 200 return emails a day.");
  await type("Company (optional)", "Northwind");
  expect(send().getAttribute("href")).toBe("mailto:hello%40tryagentclash.com?subject=Agents%20for%20our%20team&body=We%20answer%20200%20return%20emails%20a%20day.%0A%0ACompany%3A%20Northwind");
  expect(popup().textContent).toContain("Opens your email app with this text. You choose when to send it.");
  // The address stays in view as a plain link too.
  expect([...popup().querySelectorAll("a")].some(a => a.getAttribute("href") === "mailto:hello@tryagentclash.com" && a.textContent === "hello@tryagentclash.com")).toBe(true);
});

it("keeps a working link and offers to copy the text when the text is too long for a mail link", async () => {
  await render();
  await type("What work should agents take on?", "x".repeat(1990));
  expect(send().getAttribute("href")).toBe("mailto:hello%40tryagentclash.com?subject=Agents%20for%20our%20team");
  expect(popup().textContent).toContain("Your text is too long for an email link.");
  expect([...popup().querySelectorAll("button")].some(b => b.textContent === "Copy your text")).toBe(true);
});

it("uses the address that the API gives", async () => {
  await render("team@example.test");
  expect(send().getAttribute("href")).toBe("mailto:team%40example.test?subject=Agents%20for%20our%20team");
  expect(popup().textContent).toContain("team@example.test");
});

it("explains a failed copy when the Clipboard API is absent, and keeps the status region in the tree", async () => {
  vi.stubGlobal("navigator", { ...navigator, clipboard: undefined });
  await render();
  const status = popup().querySelector('[role="status"]')!;
  expect(status.textContent).toBe("");
  const copy = [...popup().querySelectorAll("button")].find(b => b.textContent === "Copy")!;
  await act(async () => { copy.click(); });
  expect(status.textContent).toBe("Copy is not available here. Select the text and copy it yourself.");
});
