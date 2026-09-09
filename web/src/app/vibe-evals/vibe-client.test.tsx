import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Session } from "@/lib/vibe";
import { VibeClient } from "./vibe-client";

const harness = vi.hoisted(() => ({
  params: new URLSearchParams("session=session-one"),
  token: vi.fn(async () => undefined as string | undefined),
  watch: vi.fn(),
  me: vi.fn(),
}));
vi.mock("next/navigation", () => ({ useSearchParams: () => harness.params }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({ useAccessToken: () => ({ getAccessToken: harness.token }) }));
vi.mock("@/lib/vibe", async (original) => ({ ...await original<typeof import("@/lib/vibe")>(), watchVibe: harness.watch }));
vi.mock("@/lib/api/client", () => ({ createApiClient: () => ({ get: harness.me }) }));
vi.mock("@/components/vibe/credits-dialog", () => ({ CreditsDialog: ({ workspace }: { workspace: string }) => <span>Credits for {workspace}</span> }));

let container: HTMLDivElement;
let root: Root;
let session: Session;
let snapshot: (value: Session) => void;
let requests: { path: string; method: string; body: Record<string, unknown> }[];
let respond: (path: string, options: RequestInit) => Promise<Response>;
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
async function render() { await act(async () => root.render(<VibeClient />)); }
function button(name: string) {
  const found = [...document.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.getAttribute("aria-label") === name || b.textContent?.trim() === name);
  expect(found, `button: ${name}`).toBeTruthy();
  return found!;
}
async function click(name: string) { await act(async () => button(name).click()); }
async function type(value: string, label = "Message Vibe Evals") {
  const input = container.querySelector<HTMLTextAreaElement>(`textarea[aria-label="${label}"]`)!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
function composer() { return container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Message Vibe Evals"]')!; }
function posts() { return requests.filter((r) => r.path.endsWith("/messages")); }

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  Element.prototype.scrollIntoView = vi.fn();
  harness.params = new URLSearchParams("session=session-one");
  harness.token.mockResolvedValue(undefined);
  harness.watch.mockImplementation((_id, _token, signal: AbortSignal, onSnapshot) => {
    snapshot = onSnapshot;
    return new Promise<void>((resolve) => signal.addEventListener("abort", () => resolve()));
  });
  session = { id: "session-one", revision: 1, event_cursor: 1, anonymous: true, document: { models: defaultModels, messages: [], requirements: [], artifacts: [] }, operations: [] };
  requests = [];
  respond = async (path) => path === "/config" ? json({ enabled: true, defaults: defaultModels, models: [] }) : json(session);
  vi.stubGlobal("fetch", vi.fn(async (url: string, options: RequestInit = {}) => {
    const path = new URL(url).pathname.replace("/v1/vibe", "");
    requests.push({ path, method: options.method || "GET", body: options.body ? JSON.parse(String(options.body)) : {} });
    return respond(path, options);
  }));
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it.each(["network", "server", "unknown-client", "mismatched-rejection", "unstructured-client", "idempotency-conflict"])("retries the immutable submission after a %s acknowledgement failure and newer SSE without auto-dispatch", async (failure) => {
  let accepted: string | undefined;
  let executions = 0;
  respond = async (path, options) => {
    if (!path.endsWith("/messages")) return path === "/config" ? json({ defaults: defaultModels, models: [] }) : json(session);
    if (!accepted) {
      accepted = String(options.body);
      executions++;
      session = { ...session, revision: 2, event_cursor: 2, document: { ...session.document, messages: [{ id: "persisted", role: "user", content: "Original request" }] } };
      if (failure === "server") return json({ error: { code: "internal", message: "Response interrupted" } }, 503);
      if (failure === "unknown-client") return json({ error: { code: "unknown_admission_outcome", message: "Response interrupted" } }, 400);
      if (failure === "mismatched-rejection") return json({ error: { code: "pricing_unavailable", message: "Unexpected error status" } }, 400);
      if (failure === "unstructured-client") return json({}, 400);
      if (failure === "idempotency-conflict") return json({ error: { code: "idempotency_conflict", message: "An operation already uses this ID" } }, 409);
      throw new TypeError("Network acknowledgement lost");
    }
    return String(options.body) === accepted ? json({ id: "original-operation" }, 202) : json({ error: { code: "idempotency_conflict", message: "Different submission" } }, 409);
  };
  await render();
  await type("Original request");
  await click("Send message");
  await act(async () => snapshot(structuredClone(session)));
  expect(posts()).toHaveLength(1);
  await type("My next request");
  await click("Retry submission");
  expect(posts()).toHaveLength(2);
  expect(posts()[1].body).toEqual(posts()[0].body);
  expect(executions).toBe(1);
  expect(composer().value).toBe("My next request");
  expect(container.textContent).not.toContain("Different submission");
});

it.each([
  { name: "rate", failures: [[429, "rate_limit"]] },
  { name: "auth", failures: [[403, "forbidden"]] },
  { name: "profile", failures: [[503, "pricing_unavailable"]] },
  { name: "rate/auth/profile", failures: [[429, "rate_limit"], [403, "forbidden"], [503, "pricing_unavailable"]] },
] as const)("retains earlier uncertainty through $name rejection until explicit idempotent success", async ({ failures }) => {
  acceptedDraft();
  const ids = vi.spyOn(crypto, "randomUUID");
  const bodies: string[] = [];
  const admissions = new Map<string, string>();
  const selected = { ...defaultModels, assistant: "openai/gpt-4o-mini", target: "openai/gpt-4.1" };
  respond = async (path, options) => {
    if (path === "/config") return json({ defaults: defaultModels, models: [defaultModels.target, selected.assistant, selected.target].map((id) => ({ id, name: id })) });
    if (!path.endsWith("/messages")) return json(session);
    const body = String(options.body);
    bodies.push(body);
    const rejection = failures[bodies.length - 2];
    if (rejection) {
      const [status, code] = rejection;
      return json({ error: { code, message: `Blocked by ${code}` } }, status);
    }
    const sub = JSON.parse(body);
    const existing = admissions.get(sub.client_id);
    if (existing) return existing === body ? json({ id: "original-operation" }, 202) : json({ error: { code: "idempotency_conflict", message: "Changed original payload" } }, 409);
    admissions.set(sub.client_id, body);
    session = {
      ...session, revision: 2, event_cursor: 2,
      document: { ...session.document, models: selected, messages: [{ id: sub.client_id, role: "user", content: sub.content }] },
      operations: [{ id: "original-operation", kind: "message", state: "COMPLETED", billing: "SETTLED", models: selected, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }],
    };
    throw new TypeError("Network acknowledgement lost");
  };
  await render();
  await selectModel("Assistant", selected.assistant);
  await selectModel("Agent", selected.target);
  await type("Original request");
  await click("Send message");
  expect(posts()[0].body).toEqual({ client_id: expect.any(String), revision: 1, kind: "message", content: "Original request", models: selected, artifact_id: "artifact-one" });
  // A later snapshot must not replace any of the original request's fields.
  session = {
    ...session, revision: 3, event_cursor: 3,
    document: { ...session.document, models: defaultModels, active_artifact_id: "artifact-two", artifacts: [...session.document.artifacts, { ...session.document.artifacts[0], id: "artifact-two" }] },
  };
  await act(async () => snapshot(structuredClone(session)));
  await type("My next request");
  expect(posts()).toHaveLength(1);
  for (const [index, [, code]] of failures.entries()) {
    await click("Retry submission");
    expect(container.textContent).toContain(`Blocked by ${code}`);
    expect(button("Send message").disabled).toBe(true);
    expect(button("Import an evaluation").disabled).toBe(true);
    expect(button("Keep it").disabled).toBe(true);
    for (const label of ["Assistant", "Agent"]) expect(container.querySelector<HTMLSelectElement>(`select[aria-label="${label} model"]`)!.disabled).toBe(true);
    expect(button("Retry submission").disabled).toBe(false);
    await act(async () => composer().dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true })));
    await act(async () => snapshot(structuredClone(session)));
    expect(posts()).toHaveLength(index + 2);
    expect(composer().value).toBe("My next request");
  }
  await click("Retry submission");
  expect(posts()).toHaveLength(failures.length + 2);
  expect(bodies.every((body) => body === bodies[0])).toBe(true);
  expect(posts().every((post) => post.path === "/sessions/session-one/messages")).toBe(true);
  expect(ids).toHaveBeenCalledTimes(1);
  expect(admissions.size).toBe(1);
  expect(session.operations).toHaveLength(1);
  expect(composer().value).toBe("My next request");
  expect(button("Send message").disabled).toBe(false);
  expect(button("Keep it").disabled).toBe(false);
  expect(container.textContent).not.toContain("Retry submission");
  expect(container.textContent).not.toContain("Changed original payload");
});

it.each(["A different next message", "Original request"])("preserves text typed during a pending POST, including retyping %s", async (next) => {
  const response = deferred<Response>();
  respond = async (path) => path.endsWith("/messages") ? response.promise : path === "/config" ? json({ defaults: defaultModels, models: [] }) : json(session);
  await render();
  await type("Original request");
  await click("Send message");
  expect(posts()).toHaveLength(1);
  await type("");
  await type(next);
  await act(async () => response.resolve(json({ id: "accepted-operation" }, 202)));
  expect(composer().value).toBe(next);
});

it.each(["pending", "failed"])("never creates a replacement session while the URL session GET is %s", async (outcome) => {
  const response = deferred<Response>();
  respond = async (path) => path === "/config" ? json({ defaults: defaultModels, models: [] }) : response.promise;
  await render();
  if (outcome === "failed") await act(async () => response.reject(new Error("Cannot load this conversation")));
  await type("Do not create another conversation");
  // Exercise keyboard submission as well as the button gate.
  await act(async () => composer().dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true })));
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  expect(button("Send message").disabled).toBe(true);
  if (outcome === "pending") {
    await act(async () => response.resolve(json(session)));
    expect(button("Send message").disabled).toBe(false);
  } else {
    expect(container.textContent).toContain("Cannot load this conversation");
    respond = async () => json(session);
    await click("Retry loading conversation");
    expect(button("Send message").disabled).toBe(false);
    expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  }
});

function acceptedDraft() {
  session.document.artifacts = [{ id: "artifact-one", title: "Support agent", agent_prompt: "Original policy", blueprint: { cases: [{ key: "refund" }] }, accepted: true, source_message_id: "message-one" }];
  session.document.active_artifact_id = "artifact-one";
}

it("requires saving edited instructions as a draft then explicitly accepting before check, playground or Keep", async () => {
  acceptedDraft();
  const original = structuredClone(session.document.artifacts[0]);
  respond = async (path, options) => {
    if (path === "/config") return json({ defaults: defaultModels, models: [] });
    if (options.method === "PATCH") {
      const body = JSON.parse(String(options.body));
      if (body.revision !== session.revision) return json({ error: { code: "revision_conflict", message: "Stale edit" } }, 409);
      if (body.agent_prompt) session.document.artifacts.push({ ...original, id: "artifact-two", parent_id: original.id, agent_prompt: body.agent_prompt, accepted: false });
      else {
        session.document.artifacts.at(-1)!.accepted = true;
        session.document.active_artifact_id = "artifact-two";
      }
      session.revision++;
      session.event_cursor = session.revision;
    }
    return json(session);
  };
  await render();
  await type("A playground question", "Agent playground message");
  const instructions = container.querySelector<HTMLTextAreaElement>('aside textarea')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(instructions, "Edited policy");
    instructions.dispatchEvent(new Event("input", { bubbles: true }));
  });
  for (const name of ["Check this agent", "Send to agent", "Keep it"]) {
    await click(name);
    expect(button(name).disabled, name).toBe(true);
  }
  expect(posts()).toHaveLength(0);
  await click("Save as a new draft");
  expect(session.document.artifacts[0]).toEqual(original);
  expect(button("Check this agent").disabled).toBe(true);
  expect(button("Keep it").disabled).toBe(true);
  await click("Accept this draft");
  await click("Check this agent");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body.artifact_id).toBe("artifact-two");
});

it("clears the execution gate when closing the editor discards its unsaved text", async () => {
  acceptedDraft();
  await render();
  const instructions = container.querySelector<HTMLTextAreaElement>('aside textarea')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(instructions, "Unsaved policy");
    instructions.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await click("Close draft");
  await click("Your agent");
  expect(container.querySelector<HTMLTextAreaElement>('aside textarea')!.value).toBe("Original policy");
  await click("Check this agent");
  expect(posts()).toHaveLength(1);
});

it.each([["Confirm", "accepted"], ["Dismiss", "rejected"]] as const)("lets a conversational proposal be %s with no draft and no scorecard", async (action, status) => {
  respond = async (path, options) => {
    if (path === "/config") return json({ defaults: defaultModels, models: [] });
    if (path.endsWith("/messages")) {
      session.document.messages = [{ id: "reply", role: "assistant", content: "Would you like to explore support use cases?" }];
      session.document.requirements = [{ id: "proposal", statement: "Explore support use cases", status: "proposed", source_message_id: "reply" }];
      session.revision++;
      session.event_cursor = session.revision;
    }
    if (options.method === "PATCH") {
      const body = JSON.parse(String(options.body));
      if (body.revision !== session.revision) return json({ error: { code: "revision_conflict", message: "Stale proposal" } }, 409);
      expect(body.requirement_id).toBe("proposal");
      session.document.requirements[0].status = body.status;
      session.revision++;
      session.event_cursor = session.revision;
    }
    return json(session);
  };
  await render();
  await type("Just exploring AI");
  await click("Send message");
  expect(posts()).toHaveLength(1);
  expect(container.textContent).toContain("Would you like to explore support use cases?");
  expect(container.querySelector('article[aria-label="Evaluation scorecard"]')).toBeNull();
  expect(container.querySelector('aside[aria-label="Agent draft"]')).toBeNull();
  expect(container.textContent).toContain("Proposed · needs your confirmation");
  await click(action);
  expect(session.document.requirements[0].status).toBe(status);
  expect(container.textContent).not.toContain("Proposed · needs your confirmation");
});

function signedInWorkspace() {
  harness.token.mockResolvedValue("test-access-token");
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-one", name: "Workspace One", role: "workspace_admin" }] }] });
}
async function selectModel(label: string, value: string) {
  const input = container.querySelector<HTMLSelectElement>(`select[aria-label="${label} model"]`)!;
  await act(async () => { input.value = value; input.dispatchEvent(new Event("change", { bubbles: true })); });
}
it.each(["pricing_unavailable", "hosted_disabled", "accounting_unavailable", "trial_capacity_reached"])("allows model correction and saving after an initial explicit %s rejection", async (code) => {
  acceptedDraft();
  signedInWorkspace();
  respond = async (path, options) => {
    if (path === "/config") return json({ defaults: defaultModels, models: [{ id: defaultModels.target, name: "Mini" }, { id: "openai/gpt-4.1", name: "Full" }] });
    if (path.endsWith("/messages")) return json({ error: { code, message: "Execution was not admitted" } }, 503);
    if (path.endsWith("/save")) {
      const body = JSON.parse(String(options.body));
      Object.assign(session, { workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: body.artifact_id, saved_models: body.models });
      session.document.models = body.models;
      session.revision++;
      session.event_cursor = session.revision;
      return json({ draft_id: "canonical-one", workspace_id: "workspace-one" });
    }
    return json(session);
  };
  await render();
  await type("Keep this message");
  await click("Send message");
  expect(container.textContent).toContain("Execution was not admitted");
  expect(composer().value).toBe("Keep this message");
  expect(posts()).toHaveLength(1);
  for (const label of ["Assistant", "Agent"]) {
    expect(container.querySelector<HTMLSelectElement>(`select[aria-label="${label} model"]`)!.disabled, label).toBe(false);
    await selectModel(label, "openai/gpt-4.1");
  }
  expect(button("Import an evaluation").disabled).toBe(false);
  expect(button("Send message").disabled).toBe(false);
  expect(button("Keep it").disabled).toBe(false);
  expect(container.textContent).not.toContain("Retry submission");
  await click("Keep it");
  await click("Save to workspace");
  const save = requests.find((r) => r.path.endsWith("/save"))!;
  expect(save.body.models).toEqual({ ...defaultModels, assistant: "openai/gpt-4.1", target: "openai/gpt-4.1" });
  expect(button("Saved").disabled).toBe(true);
  expect(container.querySelector('a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]')).not.toBeNull();
  expect(composer().value).toBe("Keep this message");
  expect(posts()).toHaveLength(1);
});

it("Keep sends the independently selected models without an inference submission", async () => {
  acceptedDraft();
  signedInWorkspace();
  respond = async (path, options) => {
    if (path === "/config") return json({ defaults: defaultModels, models: [{ id: defaultModels.target, name: "Mini" }, { id: "openai/gpt-4.1", name: "Full" }] });
    if (path.endsWith("/save")) {
      const body = JSON.parse(String(options.body));
      Object.assign(session, { workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: body.artifact_id, saved_models: body.models });
      session.document.models = body.models;
      session.revision++;
      session.event_cursor = session.revision;
      return json({ draft_id: "canonical-one", workspace_id: "workspace-one" });
    }
    return json(session);
  };
  await render();
  await selectModel("Agent", "openai/gpt-4.1");
  await click("Keep it");
  await click("Save to workspace");
  const save = requests.find((r) => r.path.endsWith("/save"))!;
  expect(save.body.models).toEqual({ ...defaultModels, target: "openai/gpt-4.1" });
  expect(save.body.artifact_id).toBe("artifact-one");
  expect(button("Saved").disabled).toBe(true);
  expect(posts()).toHaveLength(0);
});

it.each([
  { name: "confirmed", receipt: { saved_models: defaultModels }, confirmed: true },
  { name: "legacy null", receipt: { saved_models: null }, confirmed: false },
  { name: "legacy absent", receipt: {}, confirmed: false },
])("hydrates the $name saved artifact link from a session-only URL, clearing it for a new artifact", async ({ receipt, confirmed }) => {
  acceptedDraft();
  signedInWorkspace();
  Object.assign(session, { anonymous: false, workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: "artifact-one", ...receipt });
  await render();
  const href = '/workspaces/workspace-one/challenge-packs/builder/canonical-one';
  expect(container.querySelector(`a[href="${href}"]`)).not.toBeNull();
  expect(container.textContent).toContain("Credits for workspace-one");
  await click("Keep it");
  expect(button(confirmed ? "Saved" : "Save to workspace").disabled).toBe(confirmed);
  session.document.artifacts.push({ ...session.document.artifacts[0], id: "artifact-two", accepted: true });
  session.document.active_artifact_id = "artifact-two";
  session.revision++;
  session.event_cursor = session.revision;
  await act(async () => snapshot(structuredClone(session)));
  expect(document.querySelector(`a[href="${href}"]`)).toBeNull();
  expect(button("Save to workspace").disabled).toBe(false);
  expect(container.textContent).not.toContain("Your evaluation is saved.");
  expect([...document.querySelectorAll("button")].some((b) => b.textContent === "Saved")).toBe(false);
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
});
it.each([["Assistant", "assistant"], ["Agent", "target"], ["Evaluator", "evaluator"]] as const)("clears Saved when the %s model changes, retaining its canonical link after a conflict", async (label, role) => {
  acceptedDraft();
  signedInWorkspace();
  Object.assign(session, { anonymous: false, workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: "artifact-one", saved_models: defaultModels });
  respond = async (path) => path === "/config"
    ? json({ defaults: defaultModels, models: [{ id: defaultModels.target, name: "Mini" }, { id: "openai/gpt-4.1", name: "Full" }] })
    : path.endsWith("/save")
      ? json({ error: { code: "saved_model_conflict", message: "Create a new draft before saving different models." } }, 409)
      : json(session);
  await render();
  expect(container.textContent).toContain("Your evaluation is saved.");
  await selectModel(label, "openai/gpt-4.1");
  expect(container.textContent).not.toContain("Your evaluation is saved.");
  expect(container.textContent).toContain("Your current model choices differ from those recorded when this draft was saved.");
  await click("Keep it");
  await click("Save to workspace");
  const dialog = document.querySelector('[role="dialog"]')!;
  expect(dialog.textContent).toContain("Create a new draft before saving different models.");
  expect(dialog.querySelector('a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]')).not.toBeNull();
  expect(button("Save to workspace").disabled).toBe(false);
  expect(requests.filter((r) => r.path.endsWith("/save"))).toEqual([
    { path: "/sessions/session-one/save", method: "POST", body: { revision: 1, artifact_id: "artifact-one", workspace_id: "workspace-one", models: { ...defaultModels, [role]: "openai/gpt-4.1" } } },
  ]);
  await click("Close");
  await selectModel(label, defaultModels[role]);
  expect(container.textContent).toContain("Your evaluation is saved.");
  await click("Keep it");
  expect(button("Saved").disabled).toBe(true);
  expect(requests.filter((r) => r.path.endsWith("/save"))).toHaveLength(1);
  expect(posts()).toHaveLength(0);
});

it.each([
  { name: "confirmed", receipt: { saved_models: defaultModels } },
  { name: "legacy null", receipt: { saved_models: null } },
  { name: "legacy absent", receipt: {} },
])("keeps dirty instructions gated with a $name canonical association", async ({ receipt }) => {
  acceptedDraft();
  signedInWorkspace();
  Object.assign(session, { anonymous: false, workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: "artifact-one", ...receipt });
  await render();
  const selector = 'a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]';
  expect(container.querySelector(selector)).not.toBeNull();
  await type("A playground question", "Agent playground message");
  const instructions = container.querySelector<HTMLTextAreaElement>('aside textarea')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(instructions, "Unsaved policy");
    instructions.dispatchEvent(new Event("input", { bubbles: true }));
  });
  expect(container.querySelector(selector)).toBeNull();
  expect(container.textContent).not.toContain("Your evaluation is saved.");
  for (const name of ["Keep it", "Check this agent", "Send to agent"]) {
    expect(button(name).disabled).toBe(true);
    await click(name);
  }
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  expect(requests.filter((r) => r.method !== "GET")).toHaveLength(0);
  await click("Close draft");
  await click("Your agent");
  expect(container.querySelector<HTMLTextAreaElement>('aside textarea')!.value).toBe("Original policy");
  expect(container.querySelector(selector)).not.toBeNull();
  expect(button("Keep it").disabled).toBe(false);
});

it.each([
  { name: "legacy null", receipt: { saved_models: null }, notice: "Saved model choices are unknown. Your current model choices are not confirmed as saved." },
  { name: "legacy absent", receipt: {}, notice: "Saved model choices are unknown. Your current model choices are not confirmed as saved." },
  { name: "mismatched", receipt: { saved_models: defaultModels }, notice: "Your current model choices differ from those recorded when this draft was saved." },
])("preserves a known canonical link for $name models, including after a save conflict", async ({ receipt, notice }) => {
  acceptedDraft();
  signedInWorkspace();
  // Store.GetSession retains saved_artifact_id for pre-00074 rows even though
  // their model receipt is unknown. The mutable document is not a receipt.
  const selected = { ...defaultModels, target: "openai/gpt-4.1" };
  session.document.models = selected;
  Object.assign(session, { anonymous: false, workspace_id: "workspace-one", saved_draft_id: "canonical-one", saved_artifact_id: "artifact-one", ...receipt });
  const original = structuredClone(session);
  const conflict = "These instructions are already saved with different or unrecorded model choices. Open the saved draft in your workspace, or edit and accept a new draft before saving new choices.";
  respond = async (path) => path === "/config"
    ? json({ defaults: defaultModels, models: [defaultModels.target, selected.target].map((id) => ({ id, name: id })) })
    : path.endsWith("/save")
      ? json({ error: { code: "saved_model_conflict", message: conflict } }, 409)
      : json(session);
  const href = "/workspaces/workspace-one/challenge-packs/builder/canonical-one";
  const mutations = () => requests.filter((r) => r.method !== "GET");
  async function followLink(scope: ParentNode) {
    const link = scope.querySelector<HTMLAnchorElement>(`a[href="${href}"]`);
    expect(link, "known canonical draft remains navigable").not.toBeNull();
    const before = structuredClone(requests);
    // JSDOM cannot navigate. Cancel only the browser default, not UI handlers.
    document.addEventListener("click", (event) => event.preventDefault(), { once: true });
    await act(async () => link!.click());
    expect(requests).toEqual(before);
  }
  await render();
  await followLink(container);
  expect(container.textContent).toContain(notice);
  expect(container.textContent).not.toContain("Your evaluation is saved.");
  expect(mutations()).toHaveLength(0);
  await click("Keep it");
  const dialog = document.querySelector('[role="dialog"]')!;
  expect(dialog.textContent).toContain(notice);
  expect([...dialog.querySelectorAll("button")].some((b) => b.textContent === "Saved")).toBe(false);
  await followLink(dialog);
  expect(mutations()).toHaveLength(0);
  await click("Save to workspace");
  expect(dialog.textContent).toContain(conflict);
  expect(dialog.textContent).toContain(notice);
  expect(button("Save to workspace").disabled).toBe(false);
  expect([...dialog.querySelectorAll("button")].some((b) => b.textContent === "Saved")).toBe(false);
  await followLink(dialog);
  expect(mutations()).toEqual([
    { path: "/sessions/session-one/claim", method: "POST", body: {} },
    { path: "/sessions/session-one/save", method: "POST", body: { revision: 1, artifact_id: "artifact-one", workspace_id: "workspace-one", models: selected } },
  ]);
  expect(posts()).toHaveLength(0);
  expect(session).toEqual(original);
  expect(container.querySelector<HTMLSelectElement>('select[aria-label="Agent model"]')!.value).toBe(selected.target);
});

it("does not guess a saved artifact association for legacy snapshots", async () => {
  acceptedDraft();
  Object.assign(session, { workspace_id: "workspace-one", saved_draft_id: "canonical-one" });
  await render();
  expect(container.querySelector('a[href*="canonical-one"]')).toBeNull();
});
