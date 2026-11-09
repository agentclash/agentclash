import { readFileSync } from "node:fs";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { VibeConnection } from "@/lib/vibe-connection";
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
  const style = document.createElement("style");
  style.textContent = readFileSync("src/components/vibe/project-enquiry.css", "utf8");
  document.head.append(style);
  try {
    vi.stubGlobal("navigator", { ...navigator, clipboard: undefined });
    await render();
    const status = popup().querySelector('[role="status"]')!;
    expect(status.textContent).toBe("");
    expect(getComputedStyle(status).display).not.toBe("none");
    expect(getComputedStyle(status).position).toBe("absolute");
    const copy = [...popup().querySelectorAll("button")].find(b => b.textContent === "Copy")!;
    await act(async () => { copy.click(); });
    expect(status.textContent).toBe("Copy is not available here. Select the text and copy it yourself.");
    expect(getComputedStyle(status).display).not.toBe("none");
    expect(getComputedStyle(status).position).not.toBe("absolute");
  } finally { style.remove(); }
});

const token = vi.fn(async () => "visitor-token");
async function renderDirect(open = true) {
  await act(async () => root.render(<VibeConnection.Provider value={{ token, contact: { email: "team@example.test", available: true, direct: true } }}><TalkToUs open={open} onOpenChange={onOpenChange} email="team@example.test" /></VibeConnection.Provider>));
}
const submit = () => popup().querySelector('button[type="submit"]') as HTMLButtonElement;
async function submitDirect() { await act(async () => { popup().querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); }); }

it("requires valid work and email before sending direct contact", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  await renderDirect();
  const email = popup().querySelector('input[type="email"]') as HTMLInputElement;
  expect(email.required).toBe(true);
  expect(email.autocomplete).toBe("email");
  await submitDirect();
  expect(fetch).not.toHaveBeenCalled();
  await type("What work should agents take on?", "Automate returns");
  await type("Work email", "invalid");
  await submitDirect();
  expect(popup().querySelector('[role="alert"]')?.textContent).toContain("valid email");
  expect(fetch).not.toHaveBeenCalled();
});

it("posts contact details with auth, retries the same ID and body, and focuses the receipt", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "temporary_unavailable", message: "Connection lost" } }), { status: 503 })).mockResolvedValue(new Response(JSON.stringify({ id: "receipt", status: "received" }), { status: 201 }));
  vi.stubGlobal("fetch", fetch);
  await renderDirect();
  await type("What work should agents take on?", "Automate returns");
  await type("Work email", "visitor@example.test");
  await type("Company (optional)", "Northwind");
  const honeypot = popup().querySelector('input[name="website"]') as HTMLInputElement;
  expect(honeypot.tabIndex).toBe(-1);
  expect(honeypot.getAttribute("aria-hidden")).toBe("true");
  await submitDirect();
  expect(popup().querySelector('[role="alert"]')?.textContent).toContain("Connection lost");
  expect(popup().querySelector('a[href="mailto:team@example.test"]')).not.toBeNull();
  expect((popup().querySelector('input[type="email"]') as HTMLInputElement).value).toBe("visitor@example.test");
  await submitDirect();
  expect(fetch).toHaveBeenCalledTimes(2);
  const [url, init] = fetch.mock.calls[0];
  expect(url).toMatch(/\/v1\/vibe\/enquiries$/);
  expect(new Headers(init.headers).get("Authorization")).toBe("Bearer visitor-token");
  expect(init.method).toBe("POST");
  const body = JSON.parse(init.body);
  expect(body).toEqual({ client_id: expect.stringMatching(/^[0-9a-f-]{36}$/), work: "Automate returns", email: "visitor@example.test", company: "Northwind", website: "" });
  expect(fetch.mock.calls[1][1].body).toBe(init.body);
  const receipt = popup().querySelector('.vibe-enquiry-receipt[role="status"]')!;
  expect(receipt.textContent).toBe("Sent. We will reply to visitor@example.test.");
  expect(document.activeElement).toBe(receipt);
  const close = [...popup().querySelectorAll("button")].find(b => b.textContent === "Close")!;
  await act(async () => close.click());
  expect(onOpenChange).toHaveBeenCalledWith(false);
  await renderDirect(false); await renderDirect();
  await type("What work should agents take on?", "Next task"); await type("Work email", "visitor@example.test");
  await submitDirect();
  expect(JSON.parse(fetch.mock.calls[2][1].body).client_id).not.toBe(body.client_id);
});

it("disables direct send while awaiting the response", async () => {
  let resolve!: (value: Response) => void;
  const fetch = vi.fn(() => new Promise<Response>(done => { resolve = done; }));
  vi.stubGlobal("fetch", fetch);
  await renderDirect();
  await type("What work should agents take on?", "Automate returns"); await type("Work email", "visitor@example.test");
  await submitDirect();
  expect(submit().disabled).toBe(true);
  await submitDirect(); expect(fetch).toHaveBeenCalledTimes(1);
  await act(async () => resolve(new Response(JSON.stringify({ id: "receipt", status: "received" }), { status: 201 })));
  expect(popup().textContent).toContain("Sent. We will reply to visitor@example.test.");
});

it("allows correction after a definite rejection and keeps the same dialog ID", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "invalid_request", message: "Use another reply address", admission: "rejected" } }), { status: 400 })).mockResolvedValue(new Response(JSON.stringify({ id: "receipt", status: "received" }), { status: 201 }));
  vi.stubGlobal("fetch", fetch);
  await renderDirect();
  await type("What work should agents take on?", "Automate returns"); await type("Work email", "visitor@example.test");
  await submitDirect();
  expect(popup().querySelector('[role="alert"]')?.textContent).toBe("Use another reply address");
  expect((popup().querySelector('input[type="email"]') as HTMLInputElement).disabled).toBe(false);
  await type("Work email", "corrected@example.test"); await submitDirect();
  const first = JSON.parse(fetch.mock.calls[0][1].body), second = JSON.parse(fetch.mock.calls[1][1].body);
  expect(second.client_id).toBe(first.client_id);
  expect(second.email).toBe("corrected@example.test");
  expect(popup().querySelector('.vibe-enquiry-receipt')?.textContent).toContain("corrected@example.test");
});

it("keeps the mailto flow when direct contact is unavailable", async () => {
  await act(async () => root.render(<VibeConnection.Provider value={{ token, contact: { email: "team@example.test", available: false, direct: true } }}><TalkToUs open onOpenChange={onOpenChange} email="team@example.test" /></VibeConnection.Provider>));
  expect(send().getAttribute("href")).toBe("mailto:team%40example.test?subject=Agents%20for%20our%20team");
  expect(popup().querySelector('input[type="email"]')).toBeNull();
});


it("keeps the draft and mailto mode when config arrives while the dialog is open", async () => {
  async function config(direct: boolean, open = true) {
    await act(async () => root.render(<VibeConnection.Provider value={{ token, contact: { email: "team@example.test", available: true, direct } }}><TalkToUs open={open} onOpenChange={onOpenChange} email="team@example.test" /></VibeConnection.Provider>));
  }
  await config(false, false);
  await config(false);
  await type("What work should agents take on?", "Automate returns");
  await type("Company (optional)", "Northwind");
  await config(true);
  expect((popup().querySelector("textarea") as HTMLTextAreaElement).value).toBe("Automate returns");
  expect(send().getAttribute("href")).toContain("body=Automate%20returns%0A%0ACompany%3A%20Northwind");
  expect(popup().querySelector('input[type="email"]')).toBeNull();
  await config(true, false);
  await config(true);
  expect(popup().querySelector('input[type="email"]')).not.toBeNull();
  expect((popup().querySelector("textarea") as HTMLTextAreaElement).value).toBe("Automate returns");
  expect((popup().querySelector('input[autocomplete="organization"]') as HTMLInputElement).value).toBe("Northwind");
});


it("wraps a receipt containing a 64-character reply local part", async () => {
  const style = document.createElement("style");
  style.textContent = readFileSync("src/components/vibe/project-enquiry.css", "utf8");
  document.head.append(style);
  try {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "receipt", status: "received" }), { status: 201 })));
    await renderDirect();
    const email = `${"a".repeat(64)}@example.test`;
    await type("What work should agents take on?", "Automate returns");
    await type("Work email", email);
    await submitDirect();
    const receipt = popup().querySelector(".vibe-enquiry-receipt")!;
    expect(receipt.textContent).toContain(email);
    expect(getComputedStyle(receipt).overflowWrap).toBe("anywhere");
  } finally { style.remove(); }
});
