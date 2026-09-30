import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, VibeError, vibeFetch, type Artifact, type Session } from "@/lib/vibe";
import { VibeConnection } from "@/lib/vibe-connection";
import { enquiryEmailLink, projectSummary } from "@/lib/vibe-enquiries";
import { ProjectEnquiry } from "./project-enquiry";

vi.mock("@/lib/vibe", async importOriginal => ({ ...await importOriginal<typeof import("@/lib/vibe")>(), vibeFetch: vi.fn() }));
const artifact: Artifact = { id: "version", title: "Notes assistant", accepted: true, source_message_id: "brief", agent_prompt: "Summarize notes", blueprint: { cases: [{ input: "PRIVATE CASE" }] } };
const session: Session = { id: "project", revision: 4, anonymous: true, document: { messages: [{ id: "brief", role: "user", content: "PRIVATE CHAT" }], requirements: [], artifacts: [artifact], models: defaultModels }, operations: [] };
const storageKey = "vibe-enquiry:project:version";
const draft = { summary: "My reviewed project summary & special characters", email: "person@example.test", name: "", company: "", source: { artifact_id: "version", revision: 3 } };
let node: HTMLDivElement, root: Root;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); sessionStorage.clear(); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(available = true, current = session, email = available ? "team@example.test" : "") {
  await act(async () => root.render(<VibeConnection.Provider value={{ token: async () => undefined, contact: { available, email } }}><ProjectEnquiry session={current} artifact={artifact} /></VibeConnection.Provider>));
}
const popup = () => document.querySelector('[role="dialog"]') as HTMLElement | null;
const form = () => popup()!.querySelector("form")!;
async function click(label: string) {
  const button = Array.from(document.querySelectorAll("button")).find(b => b.textContent === label);
  expect(button, `button ${label}`).toBeTruthy();
  await act(async () => button!.click());
}
async function submit() { await act(async () => form().dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }))); }
async function close() { await click("Close"); }

it("opens a review dialog without sending and hides contact fields when no recipient exists", async () => {
  await render(false); await click("Discuss this with AgentClash");
  expect(popup()).toBeTruthy();
  expect(popup()!.textContent).toContain("Contact isn’t set up yet");
  expect(popup()!.querySelector('input[type="email"]')).toBeNull();
  expect(popup()!.querySelector('button[type="submit"]')).toBeNull();
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
  const summary = projectSummary(session, artifact);
  expect(summary).not.toContain("PRIVATE CHAT"); expect(summary).not.toContain("PRIVATE CASE");
  expect(summary).toContain("Business systems are not connected");
  await close();
  expect(popup()).toBeNull();
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
});

it("offers direct email without collecting contact fields when the form is unavailable", async () => {
  await render(false, session, "team@example.test"); await click("Discuss this with AgentClash");
  expect(popup()!.textContent).toContain("The contact form isn’t available");
  expect(popup()!.querySelectorAll("input")).toHaveLength(0);
  expect(popup()!.querySelector('a[href^="mailto:"]')).toBeTruthy();
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
});

it("restores reviewed edits after close, reopen and reload without refreshing from newer props", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify(draft));
  await render(); await click("Discuss this with AgentClash");
  expect(popup()!.querySelector("textarea")!.value).toBe(draft.summary);
  await close(); await render(true, { ...session, revision: 9 }); await click("Discuss this with AgentClash");
  expect(popup()!.querySelector("textarea")!.value).toBe(draft.summary);
  expect(new URL(popup()!.querySelector("a")!.href).searchParams.get("body")).toBe(draft.summary);
  await act(async () => root.unmount()); root = createRoot(node);
  await render(true, { ...session, revision: 9 }); await click("Discuss this with AgentClash");
  expect(popup()!.querySelector("textarea")!.value).toBe(draft.summary);
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
});

it("shows validation only after submit and preserves the draft after a 4xx", async () => {
  await render(); await click("Discuss this with AgentClash");
  expect(popup()!.querySelector('[role="alert"]')).toBeNull();
  await submit();
  expect(popup()!.querySelector('[role="alert"]')!.textContent).toContain("valid email");
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
  const email = popup()!.querySelector('input[type="email"]')!;
  await act(async () => { (email as HTMLInputElement).value = "person@example.test"; email.dispatchEvent(new Event("input", { bubbles: true })); });
  // Use the stored draft for a server-side validation failure without changing request identity.
  sessionStorage.setItem(storageKey, JSON.stringify(draft));
  await act(async () => root.unmount()); root = createRoot(node);
  await render(); await click("Discuss this with AgentClash");
  vi.mocked(vibeFetch).mockRejectedValueOnce(new VibeError("invalid_request", "Please review the email.", 400));
  await submit();
  expect(popup()!.querySelector('[role="alert"]')!.textContent).toContain("Please review the email.");
  expect(JSON.parse(sessionStorage.getItem(storageKey)!).client_id).toBeUndefined();
  expect(popup()!.querySelector("textarea")!.value).toBe(draft.summary);
});

it("reuses the exact body after a lost acknowledgement and reload", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify(draft));
  vi.mocked(vibeFetch).mockRejectedValueOnce(new Error("Connection lost"));
  await render(); await click("Discuss this with AgentClash"); await submit();
  const first = vi.mocked(vibeFetch).mock.calls[0];
  expect(first[0]).toBe("/sessions/project/enquiries");
  const body = first[2]!.body as string;
  expect(JSON.parse(body).source.revision).toBe(3);
  expect(sessionStorage.getItem(storageKey)).toBe(body);
  await close();
  expect(vi.mocked(vibeFetch)).toHaveBeenCalledTimes(1);
  await act(async () => root.unmount()); root = createRoot(node);
  await render(); await click("Discuss this with AgentClash");
  expect(popup()!.querySelector("textarea")!.disabled).toBe(true);
  vi.mocked(vibeFetch).mockResolvedValueOnce({ id: "receipt", status: "received" });
  await submit();
  expect(vi.mocked(vibeFetch).mock.calls[1][2]!.body).toBe(body);
  expect(popup()!.textContent).toContain("Enquiry received. The team notification is queued.");
  expect(popup()!.textContent).not.toContain("delivered");
});

it("does not truncate long summaries into an email URL", () => {
  expect(enquiryEmailLink("team@example.test", "long summary ".repeat(1000))).toBeUndefined();
});
