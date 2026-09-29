import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, vibeFetch, type Artifact, type Session } from "@/lib/vibe";
import { VibeConnection } from "@/lib/vibe-connection";
import { enquiryEmailLink, projectSummary } from "@/lib/vibe-enquiries";
import { ProjectEnquiry } from "./project-enquiry";

vi.mock("@/lib/vibe", async importOriginal => ({ ...await importOriginal<typeof import("@/lib/vibe")>(), vibeFetch: vi.fn() }));
const artifact: Artifact = { id: "version", title: "Notes assistant", accepted: true, source_message_id: "brief", agent_prompt: "Summarize notes", blueprint: { cases: [{ input: "PRIVATE CASE" }] } };
const session: Session = { id: "project", revision: 4, anonymous: true, document: { messages: [{id:"brief",role:"user",content:"PRIVATE CHAT"}], requirements: [], artifacts: [artifact], models: defaultModels }, operations: [] };
const storageKey = "vibe-enquiry:project:version";
const draft = { summary: "My reviewed project summary & special characters", email: "person@example.test", name: "", company: "", source: { artifact_id: "version", revision: 3 } };
let node: HTMLDivElement, root: Root;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.clearAllMocks(); sessionStorage.clear(); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(available = true, current = session) {
  await act(async () => root.render(<VibeConnection.Provider value={{ token: async () => undefined, contact: { available, email: available ? "team@example.test" : "" } }}><ProjectEnquiry session={current} artifact={artifact} /></VibeConnection.Provider>));
}
async function click(label: string) { await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === label)!.click()); }
async function submit() { await act(async () => node.querySelector("form")!.dispatchEvent(new Event("submit", {bubbles:true,cancelable:true}))); }

it("opening an unconfigured form sends nothing and omits raw evidence", async () => {
  await render(false); await click("Discuss this with AgentClash");
  expect(node.textContent).toContain("Contact isn’t set up yet");
  expect(node.querySelector('button[type="submit"]')).toBeNull();
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
  const summary = projectSummary(session, artifact);
  expect(summary).not.toContain("PRIVATE CHAT"); expect(summary).not.toContain("PRIVATE CASE");
  expect(summary).toContain("Business systems are not connected");
});

it("restores an editable reviewed draft without replacing it with a newer project", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify(draft));
  await render(); await click("Discuss this with AgentClash");
  expect(node.querySelector("textarea")!.value).toBe(draft.summary);
  expect(node.querySelector("textarea")!.disabled).toBe(false);
  await render(true, {...session, revision: 9});
  expect(node.querySelector("textarea")!.value).toBe(draft.summary);
  expect(new URL(node.querySelector("a")!.href).searchParams.get("body")).toBe(draft.summary);
  expect(vi.mocked(vibeFetch)).not.toHaveBeenCalled();
});

it("reuses the exact submitted body after a lost acknowledgement and reload", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify(draft));
  vi.mocked(vibeFetch).mockRejectedValueOnce(new Error("Connection lost"));
  await render(); await click("Discuss this with AgentClash"); await submit();
  const first = vi.mocked(vibeFetch).mock.calls[0];
  expect(first[0]).toBe("/sessions/project/enquiries");
  const body = first[2]!.body as string;
  expect(JSON.parse(body).source.revision).toBe(3);
  expect(sessionStorage.getItem(storageKey)).toBe(body);
  await act(async () => root.unmount()); root = createRoot(node);
  await render(); await click("Discuss this with AgentClash");
  expect(node.querySelector("textarea")!.disabled).toBe(true);
  vi.mocked(vibeFetch).mockResolvedValueOnce({ id: "receipt", status: "received" });
  await submit();
  expect(vi.mocked(vibeFetch).mock.calls[1][2]!.body).toBe(body);
  expect(node.textContent).toContain("Enquiry received. The team notification is queued.");
  expect(node.textContent).not.toContain("delivered");
});

it("does not truncate long summaries into an email URL", () => {
  expect(enquiryEmailLink("team@example.test", "long summary ".repeat(1000))).toBeUndefined();
});
